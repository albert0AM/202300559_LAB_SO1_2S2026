#include "mem_info.h"
#include <linux/mm.h>   
#include <linux/swap.h> 

#define PAGES_TO_KB(pages) (((unsigned long)(pages)) << (PAGE_SHIFT - 10))

unsigned long print_mem_global(struct seq_file *m)
{
    struct sysinfo si;
    unsigned long total_kb;

    si_meminfo(&si);

    total_kb = PAGES_TO_KB(si.totalram);

    seq_printf(m, "=== MEMORIA GLOBAL ===\n");
    seq_printf(m, "RAM Total: %lu kB\n", total_kb);
    seq_printf(m, "RAM Libre: %lu kB\n", PAGES_TO_KB(si.freeram));
    seq_printf(m, "RAM Usada: %lu kB\n",
               total_kb - PAGES_TO_KB(si.freeram));
    seq_printf(m, "\n");

    return total_kb;
}
