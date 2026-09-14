#ifndef MEM_INFO_H
#define MEM_INFO_H

#include <linux/seq_file.h>

// Imprime en el seq_file la seccion de memoria global del sistema
void print_mem_global(struct seq_file *m);

#endif // MEM_INFO_H