package ebpfkill

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/cilium/ebpf/link"
	"github.com/cilium/ebpf/ringbuf"
)

type Event struct {
	CallerPID uint32
	TargetPID int64
	Sig       int64
	Comm      string
}

type Monitor struct {
	objs killmonitorObjects
	tp   link.Link
	rd   *ringbuf.Reader
}

func Start() (*Monitor, error) {
	var objs killmonitorObjects
	if err := loadKillmonitorObjects(&objs, nil); err != nil {
		return nil, fmt.Errorf("no se pudo cargar el programa eBPF: %w", err)
	}

	tp, err := link.Tracepoint("syscalls", "sys_enter_kill", objs.TraceKill, nil)
	if err != nil {
		objs.Close()
		return nil, fmt.Errorf("no se pudo enganchar al tracepoint: %w", err)
	}

	rd, err := ringbuf.NewReader(objs.Events)
	if err != nil {
		tp.Close()
		objs.Close()
		return nil, fmt.Errorf("no se pudo abrir el ring buffer: %w", err)
	}

	return &Monitor{objs: objs, tp: tp, rd: rd}, nil
}

func (m *Monitor) Close() {
	m.rd.Close()
	m.tp.Close()
	m.objs.Close()
}

type rawEvent struct {
	CallerPID uint32
	_         [4]byte
	TargetPID int64
	Sig       int64
	Comm      [16]byte
}

func (m *Monitor) Events() <-chan Event {
	ch := make(chan Event, 100)
	go func() {
		defer close(ch)
		for {
			// Timeout de 500ms para no bloquearse indefinidamente
			// en kernels donde el wakeup de epoll del ring buffer falla.
			m.rd.SetDeadline(time.Now().Add(500 * time.Millisecond))

			record, err := m.rd.Read()
			if err != nil {
				if errors.Is(err, ringbuf.ErrClosed) {
					log.Printf("[ebpf-monitor] Ring buffer cerrado, saliendo.")
					return
				}
				// Timeout o error transitorio — reintentamos
				continue
			}

			var raw rawEvent
			if err := binary.Read(bytes.NewReader(record.RawSample), binary.LittleEndian, &raw); err != nil {
				log.Printf("[ebpf-monitor] ERROR parseando evento: %v", err)
				continue
			}

			log.Printf("[ebpf-debug] evento: CallerPID=%d TargetPID=%d Sig=%d Comm=%q",
				raw.CallerPID, raw.TargetPID, raw.Sig, string(bytes.TrimRight(raw.Comm[:], "\x00")))

			ch <- Event{
				CallerPID: raw.CallerPID,
				TargetPID: raw.TargetPID,
				Sig:       raw.Sig,
				Comm:      string(bytes.TrimRight(raw.Comm[:], "\x00")),
			}
		}
	}()
	return ch
}
