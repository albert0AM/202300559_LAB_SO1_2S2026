// proc_info.c
// Logica relacionada al recorrido de procesos del sistema via task_struct.
#include "proc_info.h"
#include <linux/sched.h>         // struct task_struct
#include <linux/sched/signal.h>  // for_each_process
#include <linux/sched/mm.h>      // get_task_mm, mmput
#include <linux/rcupdate.h>      // rcu_read_lock/unlock
#include <linux/mm.h>            // PAGE_SHIFT, get_mm_rss

#define PAGES_TO_KB(pages) (((unsigned long)(pages)) << (PAGE_SHIFT - 10))

void print_processes(struct seq_file *m)
{
    struct task_struct *task;

    seq_printf(m, "=== PROCESOS ===\n");

    // for_each_process recorre la lista enlazada de todos los task_struct.
    rcu_read_lock();
    for_each_process(task) {
        struct mm_struct *mm;
        unsigned long vsz_kb = 0, rss_kb = 0;

        // get_task_mm toma una referencia al mm_struct del proceso.
        mm = get_task_mm(task);
        if (!mm) {
            continue;
        }

        vsz_kb = PAGES_TO_KB(mm->total_vm);
        rss_kb = PAGES_TO_KB(get_mm_rss(mm));

        mmput(mm);

        seq_printf(m,
            "PID: %d | Nombre: %s | VSZ: %lu kB | RSS: %lu kB | utime: %llu | stime: %llu\n",
            task->pid,
            task->comm,
            vsz_kb,
            rss_kb,
            (unsigned long long)task->utime,
            (unsigned long long)task->stime);
    }
    rcu_read_unlock();
}