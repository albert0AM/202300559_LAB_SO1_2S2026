package ebpfkill

import (
	"sync"
	"time"
)


type Registry struct {
	mu      sync.Mutex
	waiters map[int64]chan Event
}

// NewRegistry crea un registro vacio.
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
		default: // el canal ya tiene un valor o nadie lo esta leyendo
		}
	}
}

func (r *Registry) WaitFor(pid int64, timeout time.Duration) (Event, bool) {
	ch := make(chan Event, 1)

	r.mu.Lock()
	r.waiters[pid] = ch
	r.mu.Unlock()

	defer func() {
		r.mu.Lock()
		delete(r.waiters, pid)
		r.mu.Unlock()
	}()

	select {
	case ev := <-ch:
		return ev, true
	case <-time.After(timeout):
		return Event{}, false
	}
}