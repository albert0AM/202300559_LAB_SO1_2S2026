#include "proc_info.h"
#include <linux/sched.h>         
#include <linux/sched/signal.h>  
#include <linux/sched/mm.h>      
#include <linux/sched/task.h>    
#include <linux/rcupdate.h>      
#include <linux/mm.h>           
#include <linux/slab.h>          
#include <linux/string.h>       


#define PAGES_TO_KB(pages) (((unsigned long)(pages)) << (PAGE_SHIFT - 10))

#define MAX_TASKS 4096

#define CMDLINE_BUF_SIZE 256

void print_processes(struct seq_file *m, unsigned long total_ram_kb)
{
    struct task_struct *task;
    struct task_struct **task_list;
    int count = 0;
    int i;

    seq_printf(m, "=== PROCESOS ===\n");

    task_list = kmalloc_array(MAX_TASKS, sizeof(struct task_struct *), GFP_KERNEL);
    if (!task_list) {
        seq_printf(m, "Error: no se pudo reservar memoria temporal\n");
        return;
    }

    // --- Pasada 1: recolectar referencias seguras (no duerme)---
    rcu_read_lock();
    for_each_process(task) {
        if (count >= MAX_TASKS)
            break; 
        get_task_struct(task); 
        task_list[count++] = task;
    }
    rcu_read_unlock();

    // --- Pasada 2: procesar cada uno fuera del lock (si puede dormir) ---
    for (i = 0; i < count; i++) {
        struct task_struct *t = task_list[i];
        struct mm_struct *mm;
        unsigned long vsz_kb = 0, rss_kb = 0;
        char cmdline[CMDLINE_BUF_SIZE];
        int cmdline_len = 0;

        mm = get_task_mm(t);
        if (!mm) {
            put_task_struct(t);
            continue;
        }

        vsz_kb = PAGES_TO_KB(mm->total_vm);
        rss_kb = PAGES_TO_KB(get_mm_rss(mm));

        cmdline[0] = '\0';
        if (mm->arg_end > mm->arg_start) {
            unsigned long len = mm->arg_end - mm->arg_start;
            if (len > CMDLINE_BUF_SIZE - 1)
                len = CMDLINE_BUF_SIZE - 1;

            cmdline_len = access_process_vm(t, mm->arg_start,
                                             cmdline, len, FOLL_FORCE);
            if (cmdline_len > 0) {
                int j;
                cmdline[cmdline_len] = '\0';
                for (j = 0; j < cmdline_len - 1; j++) {
                    if (cmdline[j] == '\0')
                        cmdline[j] = ' ';
                }
            }
        }

        if (cmdline_len <= 0) {
            strscpy(cmdline, t->comm, sizeof(cmdline));
        }

        mmput(mm);
        {
            unsigned long pct_x100 = 0;
            if (total_ram_kb > 0)
                pct_x100 = (rss_kb * 10000UL) / total_ram_kb;

            seq_printf(m,
                "PID: %d | Nombre: %s | Cmd: %s | VSZ: %lu kB | RSS: %lu kB | %%MEM: %lu.%02lu%% | utime: %llu | stime: %llu\n",
                t->pid,
                t->comm,
                cmdline,
                vsz_kb,
                rss_kb,
                pct_x100 / 100, pct_x100 % 100,
                (unsigned long long)t->utime,
                (unsigned long long)t->stime);
        }

        put_task_struct(t);
    }

    kfree(task_list);
}
