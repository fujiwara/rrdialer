package rrdialer

type Locker struct {
	expire chan (struct{})
}

func NewLocker() *Locker {
	return &Locker{
		expire: make(chan struct{}, 1),
	}
}

func (l *Locker) Lock() bool {
	select {
	case l.expire <- struct{}{}:
		return true
	default:
		return false
	}
}

func (l *Locker) Unlock() bool {
	select {
	case <-l.expire:
		return true
	default:
		return false
	}
}

func (l *Locker) IsLocked() bool {
	return len(l.expire) > 0
}
