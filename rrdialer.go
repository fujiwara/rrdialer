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

// Logger is an interface for rrdialer
type Logger interface {
	Println(v ...interface{})
	Printf(format string, v ...interface{})
}

type CheckFunc func(ctx context.Context, addr string) error

type upstream struct {
	address        string
	check          CheckFunc
	logger         Logger
	locker         *Locker
	ejectThreshold int
	checkInterval  time.Duration
	checkTimeout   time.Duration
	failed         int
}

func (u *upstream) Println(v ...interface{}) {
	if u.logger != nil {
		u.logger.Println(v...)
	}
}

func (u *upstream) Printf(format string, v ...interface{}) {
	if u.logger != nil {
		u.logger.Printf(format, v...)
	}
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
		u.Printf("upstream %s check failed: %s", u.address, err)
		if u.failed >= u.ejectThreshold {
			u.Printf("upstream %s ejected. %d times failed", u.address, u.failed)
			u.locker.Lock()
		}
	} else if u.failed > 0 {
		u.Printf("upstream %s check recovered", u.address)
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
			u.Printf("shutdown checker for upstream %s", u.address)
			return
		case <-ticker.C:
		}
		if !u.locker.IsLocked() {
			u.doCheck(ctx)
		}
	}
}

type Dialer struct {
	upstreams []*upstream
	index     uint32
	logger    Logger
}

type Option struct {
	EjectThreshold int
	CheckInterval  time.Duration
	CheckTimeout   time.Duration
	Logger         Logger
	CheckFunc      CheckFunc
}

func NewOption() *Option {
	return &Option{
		EjectThreshold: DefaultEjectThreshold,
		CheckInterval:  DefaultCheckInterval,
		CheckTimeout:   DefaultCheckTimeout,
	}
}

// NewDialer makes a Dialer.
func NewDialer(ctx context.Context, address []string, opt *Option) *Dialer {
	if opt == nil {
		opt = NewOption()
	}
	upstreams := make([]*upstream, 0, len(address))
	for _, addr := range address {
		u := &upstream{
			address:        addr,
			locker:         NewLocker(),
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
		upstreams: upstreams,
		index:     0,
		logger:    opt.Logger,
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
	return nil, errors.New("all addresses are unavailable now")
}

// Get gets an available address from Dialer.
func (d *Dialer) Get() (string, error) {
	u, err := d.pick()
	if err != nil {
		return "", err
	}
	return u.address, nil
}

// Dial dials to a available address from Dialer via network.
func (d *Dialer) Dial(network string) (net.Conn, error) {
	ctx := context.Background()
	return d.DialContext(ctx, network)
}

// DialContext dials to a available address from Dialer via network with context.
func (d *Dialer) DialContext(ctx context.Context, network string) (net.Conn, error) {
	addr, err := d.Get()
	if err != nil {
		return nil, err
	}
	var da net.Dialer
	return da.DialContext(ctx, network, addr)
}

// DialContext dials to a available address from Dialer via network with timeout.
func (d *Dialer) DialTimeout(network string, timeout time.Duration) (net.Conn, error) {
	addr, err := d.Get()
	if err != nil {
		return nil, err
	}
	da := net.Dialer{Timeout: timeout}
	return da.Dial(network, addr)
}
