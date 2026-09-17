// +build ignore

#include <linux/types.h>
#include <linux/bpf.h>
#include <bpf/bpf_helpers.h>


#define TASK_COMM_LEN 16

struct event {
    __u32 caller_pid;  
    __s64 target_pid;  
    __s64 sig;       
    char  comm[TASK_COMM_LEN];
};


struct {
    __uint(type, BPF_MAP_TYPE_RINGBUF);
    __uint(max_entries, 256 * 1024);
} events SEC(".maps");

struct trace_event_raw_sys_enter_kill {
    unsigned long long unused; // campos comunes (offset 0-7)
    long id;                   // __syscall_nr, con padding (offset 8-15)
    long pid;                  // 1er argumento de kill(): PID objetivo (offset 16)
    long sig;                  // 2do argumento: la señal (offset 24)
};

SEC("tracepoint/syscalls/sys_enter_kill")
int trace_kill(struct trace_event_raw_sys_enter_kill *ctx)
{
    struct event *e;

    e = bpf_ringbuf_reserve(&events, sizeof(*e), 0);
    if (!e)
        return 0; 

    e->caller_pid = (__u32)(bpf_get_current_pid_tgid() >> 32);
    e->target_pid = ctx->pid;
    e->sig        = ctx->sig;
    bpf_get_current_comm(&e->comm, sizeof(e->comm));

    bpf_ringbuf_submit(e, 0);
    return 0;
}

char LICENSE[] SEC("license") = "GPL";