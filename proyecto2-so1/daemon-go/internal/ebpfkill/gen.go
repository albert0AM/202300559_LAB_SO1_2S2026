
package ebpfkill

//go:generate go run github.com/cilium/ebpf/cmd/bpf2go -cc clang -target amd64 killmonitor kill_monitor.c -- -I/usr/include
