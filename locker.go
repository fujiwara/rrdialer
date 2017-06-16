package rrdialer

type locker struct {
	expire chan (struct{})
}

func newLocker() *locker {
	return &locker{
		expire: make(chan struct{}, 1),
	}
}

func (l *locker) Lock() bool {
	select {
	case l.expire <- struct{}{}:
		return true
	default:
		return false
	}
}

func (l *locker) Unlock() bool {
	select {
	case <-l.expire:
		return true
	default:
		return false
	}
}

func (l *locker) IsLocked() bool {
	return len(l.expire) > 0
}
