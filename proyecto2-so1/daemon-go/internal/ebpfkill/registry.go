package ebpfkill

import (
	"sync"
	"time"
)

type Registry struct {
	mu      sync.Mutex
	waiters map[int64]chan Event
}

func NewRegistry() *Registry {
	return &Registry{waiters: make(map[int64]chan Event)}
}

func (r *Registry) Dispatch(ev Event) {
	r.mu.Lock()
	ch, ok := r.waiters[ev.TargetPID]
	r.mu.Unlock()

	if ok {
		select {
		case ch <- ev:
		default: 
		}
	}
}


func (r *Registry) Register(pid int64) <-chan Event {
	ch := make(chan Event, 1)
	r.mu.Lock()
	r.waiters[pid] = ch
	r.mu.Unlock()
	return ch
}


func (r *Registry) Unregister(pid int64) {
	r.mu.Lock()
	delete(r.waiters, pid)
	r.mu.Unlock()
}


func Wait(pid int64, ch <-chan Event, r *Registry, timeout time.Duration) (Event, bool) {
	defer r.Unregister(pid)

	select {
	case ev := <-ch:
		return ev, true
	case <-time.After(timeout):
		return Event{}, false
	}
}


func (r *Registry) WaitFor(pid int64, timeout time.Duration) (Event, bool) {
	ch := r.Register(pid)
	return Wait(pid, ch, r, timeout)
}
