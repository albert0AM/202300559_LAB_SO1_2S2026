// mem_info.c
// Logica relacionada a memoria global del sistema (RAM total/libre/usada).

#include "mem_info.h"
#include <linux/mm.h>   // si_meminfo, struct sysinfo
#include <linux/swap.h> // si_swapinfo 

#define PAGES_TO_KB(pages) (((unsigned long)(pages)) << (PAGE_SHIFT - 10))

void print_mem_global(struct seq_file *m)
{
    struct sysinfo si;

    si_meminfo(&si);

    seq_printf(m, "=== MEMORIA GLOBAL ===\n");
    seq_printf(m, "RAM Total: %lu kB\n", PAGES_TO_KB(si.totalram));
    seq_printf(m, "RAM Libre: %lu kB\n", PAGES_TO_KB(si.freeram));
    seq_printf(m, "RAM Usada: %lu kB\n",
               PAGES_TO_KB(si.totalram - si.freeram));
    seq_printf(m, "\n");
}