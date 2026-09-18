#include "proc_info.h"
#include <linux/sched.h>         
#include <linux/sched/signal.h>  
#include <linux/sched/mm.h>      
#include <linux/sched/task.h>    
#include <linux/rcupdate.h>      
#include <linux/mm.h>           
#include <linux/slab.h>          
#include <linux/string.h>       
#include <linux/ktime.h>
#include <linux/mutex.h>
#include <linux/math64.h>


#define PAGES_TO_KB(pages) (((unsigned long)(pages)) << (PAGE_SHIFT - 10))

#define MAX_TASKS 4096

#define CMDLINE_BUF_SIZE 256

/*
 * Tabla de muestras previas por PID para el calculo diferencial de %CPU.
 * Se indexa por PID con hash modulo CPU_HIST_SIZE. Como el daemon lee cada
 * 20s, la diferencia entre muestras es representativa del uso instantaneo.
 */
#define CPU_HIST_SIZE 1024
#define CPU_CACHE_WINDOW_NS (5ULL * 1000000000ULL)

struct cpu_sample {
	pid_t pid;
	bool  valid;
	u64   utime;
	u64   stime;
	u64   ts_ns;
	u64   pct_x100;
};

static DEFINE_MUTEX(cpu_hist_lock);
static struct cpu_sample cpu_hist[CPU_HIST_SIZE];

/*
 * Devuelve el %CPU * 100 del PID. Usa el delta de (utime+stime) respecto a la
 * muestra anterior partido entre el tiempo transcurrido. Si no hay muestra
 * previa (o los contadores retrocedieron por reuso de PID), cae al promedio
 * desde el arranque del proceso: (utime+stime) / (ahora - start_time).
 *
 * La ventana de cache evita que las re-impresiones de seq_file (cuando la
 * salida no cabe en un page y show() se vuelve a llamar) recalculen con un
 * delta casi nulo.
 */
static u64 cpu_percent_x100(pid_t pid, u64 utime, u64 stime,
			    u64 start_time, u64 now_ns)
{
	struct cpu_sample *s = &cpu_hist[(unsigned long)pid % CPU_HIST_SIZE];
	u64 total = utime + stime;
	u64 pct_x100 = 0;

	mutex_lock(&cpu_hist_lock);

	if (s->valid && s->pid == pid) {
		u64 elapsed = now_ns - s->ts_ns;

		if (elapsed < CPU_CACHE_WINDOW_NS) {
			pct_x100 = s->pct_x100;
			mutex_unlock(&cpu_hist_lock);
			return pct_x100;
		}

		if (total >= s->utime + s->stime) {
			u64 delta = total - (s->utime + s->stime);

			pct_x100 = div_u64(delta * 10000ULL, elapsed);
			s->utime = utime;
			s->stime = stime;
			s->ts_ns = now_ns;
			s->pct_x100 = pct_x100;
			mutex_unlock(&cpu_hist_lock);
			return pct_x100;
		}
	}

	/* Primera muestra del PID (o contadores reiniciados). */
	if (now_ns > start_time)
		pct_x100 = div_u64(total * 10000ULL, now_ns - start_time);

	s->pid = pid;
	s->valid = true;
	s->utime = utime;
	s->stime = stime;
	s->ts_ns = now_ns;
	s->pct_x100 = pct_x100;

	mutex_unlock(&cpu_hist_lock);
	return pct_x100;
}

void print_processes(struct seq_file *m, unsigned long total_ram_kb)
{
    struct task_struct *task;
    struct task_struct **task_list;
    int count = 0;
    int i;
    u64 now_ns;

    seq_printf(m, "=== PROCESOS ===\n");

    now_ns = ktime_get_ns();

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
            u64 cpu_x100;

            if (total_ram_kb > 0)
                pct_x100 = (rss_kb * 10000UL) / total_ram_kb;

            cpu_x100 = cpu_percent_x100(t->pid, t->utime, t->stime,
                                        t->start_time, now_ns);

            seq_printf(m,
                "PID: %d | Nombre: %s | Cmd: %s | VSZ: %lu kB | RSS: %lu kB | %%MEM: %lu.%02lu%% | %%CPU: %llu.%02llu%% | utime: %llu | stime: %llu\n",
                t->pid,
                t->comm,
                cmdline,
                vsz_kb,
                rss_kb,
                pct_x100 / 100, pct_x100 % 100,
                (unsigned long long)(cpu_x100 / 100),
                (unsigned long long)(cpu_x100 % 100),
                (unsigned long long)t->utime,
                (unsigned long long)t->stime);
        }

        put_task_struct(t);
    }

    kfree(task_list);
}
