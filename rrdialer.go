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
	DefaultEjectTimeout   = 10 * time.Second
	DefaultCheckInterval  = 5 * time.Second
	DefaultCheckTimeout   = 5 * time.Second
)

// Logger is an interface for rrdialer
type Logger interface {
	Println(v ...interface{})
	Printf(format string, v ...interface{})
}

type CheckFunc func(ctx context.Context, addr string) error

type target struct {
	address        string
	locker         *Locker
	check          CheckFunc
	logger         Logger
	ejectThreshold int
	ejectTimeout   time.Duration
	checkInterval  time.Duration
	checkTimeout   time.Duration
	failed         int
}

func (t *target) Println(v ...interface{}) {
	if t.logger != nil {
		t.logger.Println(v...)
	}
}

func (t *target) Printf(format string, v ...interface{}) {
	if t.logger != nil {
		t.logger.Printf(format, v...)
	}
}

func (t *target) doCheck(ctx context.Context) {
	checkCtx, cancel := context.WithTimeout(ctx, t.checkTimeout)
	defer cancel()
	err := t.check(checkCtx, t.address)
	if err != nil {
		select {
		case <-ctx.Done():
			// parent context has done. not an error
			return
		default:
		}
		t.failed++
		t.Printf("check failed for %s: %s", t.address, err)
		if t.failed >= t.ejectThreshold {
			t.Printf("%s will be ejected until %s", t.address, time.Now().Add(t.ejectTimeout))
			t.locker.Lock(t.ejectTimeout)
		}
	} else if t.failed > 0 {
		t.Printf("check recovered for %s", t.address)
		t.failed = 0
	}
}

func (t *target) runChecker(ctx context.Context) {
	ticker := time.NewTicker(t.checkInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			t.Printf("shutdown checker for %s", t.address)
			return
		case <-ticker.C:
		}
		if !t.locker.IsLocked() {
			t.doCheck(ctx)
		}
	}
}

type Dialer struct {
	targets []*target
	index   uint32
	logger  Logger
}

type Option struct {
	EjectThreshold int
	EjectTimeout   time.Duration
	CheckInterval  time.Duration
	CheckTimeout   time.Duration
	Logger         Logger
	CheckFunc      CheckFunc
}

func NewOption() *Option {
	return &Option{
		EjectThreshold: DefaultEjectThreshold,
		EjectTimeout:   DefaultEjectTimeout,
		CheckInterval:  DefaultCheckInterval,
		CheckTimeout:   DefaultCheckTimeout,
	}
}

// NewDialer makes a Dialer.
func NewDialer(ctx context.Context, address []string, opt *Option) *Dialer {
	if opt == nil {
		opt = NewOption()
	}
	targets := make([]*target, 0)
	for _, addr := range address {
		t := &target{
			address:        addr,
			locker:         NewLocker(),
			logger:         opt.Logger,
			check:          opt.CheckFunc,
			checkInterval:  opt.CheckInterval,
			checkTimeout:   opt.CheckTimeout,
			ejectThreshold: opt.EjectThreshold,
			ejectTimeout:   opt.EjectTimeout,
		}
		if t.check != nil {
			go t.runChecker(ctx)
		}
		targets = append(targets, t)
	}
	return &Dialer{
		targets: targets,
		index:   0,
		logger:  opt.Logger,
	}
}

func (d *Dialer) pick() (*target, error) {
	index := atomic.AddUint32(&(d.index), 1)
	targets := make([]*target, 0)
	for _, t := range d.targets {
		if t.locker.IsLocked() {
			continue
		}
		targets = append(targets, t)
	}
	if len(targets) == 0 {
		return nil, errors.New("all addresses are unavailable now")
	}
	return targets[index%uint32(len(targets))], nil
}

// Get gets an available address from Dialer.
func (d *Dialer) Get() (string, error) {
	t, err := d.pick()
	if err != nil {
		return "", err
	}
	return t.address, nil
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
