// Package ebpfkill carga el programa eBPF que confirma, a nivel de
// kernel, cuando un contenedor realmente murio (enganchado a la
// syscall kill()).
package ebpfkill

// Este comando compila kill_monitor.c a bytecode eBPF y genera un
// archivo .go con ese bytecode embebido (killmonitor_bpfel.go), listo
// para cargarse desde Go sin depender de libbpf en tiempo de ejecucion.
// Se corre UNA vez (o cada vez que cambies el .c) con: go generate ./...

//go:generate go run github.com/cilium/ebpf/cmd/bpf2go -cc clang -target amd64 killmonitor kill_monitor.c -- -I/usr/include
