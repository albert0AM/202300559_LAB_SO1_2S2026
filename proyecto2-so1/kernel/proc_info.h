#ifndef PROC_INFO_H
#define PROC_INFO_H

#include <linux/seq_file.h>

void print_processes(struct seq_file *m);

#endif // PROC_INFO_H