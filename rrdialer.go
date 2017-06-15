package rrdialer

import (
	"context"
	"errors"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

const DefaultEjectTimeout = time.Second

type Logger interface {
	Println(v ...interface{})
	Printf(format string, v ...interface{})
}

type target struct {
	address string
	locker  *Locker
}

type Dialer struct {
	Dialer       net.Dialer
	targets      []target
	index        uint32
	mu           sync.Mutex
	EjectTimeout time.Duration
	Logger       Logger
}

func NewDialer(address ...string) *Dialer {
	targets := make([]target, 0)
	for _, addr := range address {
		t := target{
			address: addr,
			locker:  NewLocker(),
		}
		targets = append(targets, t)
	}
	return &Dialer{
		targets:      targets,
		index:        0,
		EjectTimeout: DefaultEjectTimeout,
	}
}

func (d *Dialer) pick() (target, error) {
	index := atomic.AddUint32(&(d.index), 1)
	targets := make([]target, 0)
	for _, t := range d.targets {
		if t.locker.IsLocked() {
			continue
		}
		targets = append(targets, t)
	}
	if len(targets) == 0 {
		return target{}, errors.New("all addresses are unavailable now")
	}
	return targets[index%uint32(len(targets))], nil
}

func (d *Dialer) Dial(network string) (net.Conn, error) {
	for {
		t, err := d.pick()
		if err != nil {
			return nil, err
		}
		conn, err := d.Dialer.Dial(network, t.address)
		if err != nil {
			if d.Logger != nil {
				d.Logger.Printf("%s %s eject until %s", err, t.address, time.Now().Add(d.EjectTimeout))
			}
			t.locker.Lock(d.EjectTimeout)
			continue
		}
		return conn, err
	}
}

func (d *Dialer) DialContext(ctx context.Context, network string) (net.Conn, error) {
	for {
		t, err := d.pick()
		if err != nil {
			return nil, err
		}
		conn, err := d.Dialer.DialContext(ctx, network, t.address)
		if err != nil {
			if d.Logger != nil {
				d.Logger.Printf("%s: %s will be ejected until %s", time.Now().Add(d.EjectTimeout))
			}
			t.locker.Lock(d.EjectTimeout)
			continue
		}
		return conn, err
	}
}
