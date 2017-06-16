package rrdialer

import (
	"context"
	"errors"
	"net"
	"sync/atomic"
	"time"
)

const (
	DefaultEjectThreshold = 2
	DefaultCheckInterval  = 5 * time.Second
	DefaultCheckTimeout   = 5 * time.Second
)

var (
	defaultLogger = &nullLogger{}
)

// Logger is an interface for rrdialer
type Logger interface {
	Println(v ...interface{})
	Printf(format string, v ...interface{})
}

type nullLogger struct {
}

func (l nullLogger) Println(v ...interface{}) {
}

func (l nullLogger) Printf(_ string, v ...interface{}) {
}

type CheckFunc func(ctx context.Context, addr string) error

type upstream struct {
	address        string
	check          CheckFunc
	logger         Logger
	locker         *locker
	ejectThreshold int
	checkInterval  time.Duration
	checkTimeout   time.Duration
	failed         int
}

func (u *upstream) doCheck(ctx context.Context) {
	checkCtx, cancel := context.WithTimeout(ctx, u.checkTimeout)
	defer cancel()
	err := u.check(checkCtx, u.address)
	if err != nil {
		select {
		case <-ctx.Done():
			// parent context has done. not an error
			return
		default:
		}
		u.failed++
		if u.locker.IsLocked() {
			// still failing. silent return
			return
		}
		u.logger.Printf("upstream %s check failed: %s", u.address, err)
		if u.failed >= u.ejectThreshold {
			u.logger.Printf("upstream %s ejected. %d times failed", u.address, u.failed)
			u.locker.Lock()
		}
	} else if u.failed > 0 {
		u.logger.Printf("upstream %s check recovered", u.address)
		u.failed = 0
		u.locker.Unlock()
	}
}

func (u *upstream) runChecker(ctx context.Context) {
	ticker := time.NewTicker(u.checkInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			u.logger.Printf("shutdown checker for upstream %s", u.address)
			return
		case <-ticker.C:
		}
		u.doCheck(ctx)
	}
}

type Dialer struct {
	upstreams    []*upstream
	index        uint32
	logger       Logger
	nextUpstream bool
}

type Option struct {
	EjectThreshold int           // When health check failed count reached EjectThreshold, upstream will be ejected until pass health check
	CheckInterval  time.Duration // health check interval
	CheckTimeout   time.Duration // health check timeout
	Logger         Logger
	CheckFunc      CheckFunc
	NextUpstream   bool // Try next upstream when dial failed
}

// NewOption makes Option with default values.
func NewOption() *Option {
	opt := &Option{}
	return opt.setDefault()
}

func (opt *Option) setDefault() *Option {
	if opt.EjectThreshold == 0 {
		opt.EjectThreshold = DefaultEjectThreshold
	}
	if opt.CheckInterval == 0 {
		opt.CheckInterval = DefaultCheckInterval
	}
	if opt.CheckTimeout == 0 {
		opt.CheckTimeout = DefaultCheckTimeout
	}
	if opt.Logger == nil {
		opt.Logger = defaultLogger
	}
	return opt
}

// NewDialer makes a Dialer.
func NewDialer(ctx context.Context, address []string, opt *Option) *Dialer {
	if opt == nil {
		opt = NewOption()
	} else {
		opt.setDefault()
	}
	upstreams := make([]*upstream, 0, len(address))
	for _, addr := range address {
		u := &upstream{
			address:        addr,
			locker:         newLocker(),
			logger:         opt.Logger,
			check:          opt.CheckFunc,
			checkInterval:  opt.CheckInterval,
			checkTimeout:   opt.CheckTimeout,
			ejectThreshold: opt.EjectThreshold,
		}
		if u.check != nil {
			go u.runChecker(ctx)
		}
		upstreams = append(upstreams, u)
	}
	return &Dialer{
		upstreams:    upstreams,
		index:        0,
		logger:       opt.Logger,
		nextUpstream: opt.NextUpstream,
	}
}

func (d *Dialer) pick() (*upstream, error) {
	index := atomic.AddUint32(&(d.index), 1)
	n := uint32(len(d.upstreams))
	for i := index; i < index+n; i++ {
		u := d.upstreams[i%n]
		if u.locker.IsLocked() {
			continue
		}
		return u, nil
	}
	return nil, errors.New("all upstreams are unavailable")
}

func (d *Dialer) pickAll() ([]*upstream, error) {
	index := atomic.AddUint32(&(d.index), 1)
	n := uint32(len(d.upstreams))
	ups := make([]*upstream, 0, n)
	for i := index; i < index+n; i++ {
		u := d.upstreams[i%n]
		if u.locker.IsLocked() {
			continue
		}
		ups = append(ups, u)
	}
	if len(ups) == 0 {
		return nil, errors.New("all upstreams are unavailable")
	}
	return ups, nil
}

// Get gets an available address in Dialer.
func (d *Dialer) Get() (string, error) {
	u, err := d.pick()
	if err != nil {
		return "", err
	}
	return u.address, nil
}

// Dial dials to an available address in Dialer via network.
func (d *Dialer) Dial(network string) (net.Conn, error) {
	ctx := context.Background()
	return d.dial(ctx, network, 0)
}

// DialContext dials to an available address in Dialer via network with timeout.
func (d *Dialer) DialTimeout(network string, timeout time.Duration) (net.Conn, error) {
	ctx := context.Background()
	return d.dial(ctx, network, timeout)
}

// DialContext dials to an available address in Dialer via network with context.
func (d *Dialer) DialContext(ctx context.Context, network string) (net.Conn, error) {
	return d.dial(ctx, network, 0)
}

func (d *Dialer) dial(ctx context.Context, network string, timeout time.Duration) (net.Conn, error) {
	ups, err := d.pickAll()
	if err != nil {
		return nil, err
	}
	da := net.Dialer{Timeout: timeout}
	var lastError error
	for i, up := range ups {
		conn, err := da.DialContext(ctx, network, up.address)
		if err != nil {
			lastError = err
			select {
			case <-ctx.Done():
				return nil, err
			default:
			}
			if d.nextUpstream && i < len(ups)-1 {
				d.logger.Printf("connect error to upstream %s %s. trying next upstream", up.address, err)
				continue
			} else {
				return nil, err
			}
		}
		return conn, err
	}
	return nil, lastError
}
