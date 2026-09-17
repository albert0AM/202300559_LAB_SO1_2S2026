package ebpfkill

import (
	"bytes"
	"encoding/binary"
	"fmt"

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
	objs killmonitorObjects // generado por bpf2go (gen.go)
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
	_         [4]byte // padding, no se usa
	TargetPID int64
	Sig       int64
	Comm      [16]byte
}

func (m *Monitor) Events() <-chan Event {
	ch := make(chan Event, 100)
	go func() {
		defer close(ch)
		for {
			record, err := m.rd.Read()
			if err != nil {
				return 
			}

			var raw rawEvent
			if err := binary.Read(bytes.NewReader(record.RawSample), binary.LittleEndian, &raw); err != nil {
				continue // evento corrupto/incompleto, lo ignoramos
			}

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