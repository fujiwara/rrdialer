package rrdialer

import (
	"time"
)

type Locker struct {
	expire chan (interface{})
}

func NewLocker() *Locker {
	return &Locker{
		expire: make(chan interface{}, 1),
	}
}

func (l *Locker) Lock(d time.Duration) bool {
	select {
	case l.expire <- nil:
		time.AfterFunc(d, func() {
			<-l.expire
		})
		return true
	default:
	}
	return false
}

func (l *Locker) IsLocked() bool {
	return len(l.expire) > 0
}
