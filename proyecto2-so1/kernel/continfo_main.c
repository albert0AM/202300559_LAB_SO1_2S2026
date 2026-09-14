#include <linux/module.h>
#include <linux/kernel.h>
#include <linux/init.h>
#include <linux/proc_fs.h>
#include <linux/seq_file.h>
#include <linux/version.h>

#include "mem_info.h"
#include "proc_info.h"

#define CARNET "202300559"
#define PROC_NAME "continfo_pr2_so1_" CARNET

static int continfo_show(struct seq_file *m, void *v)
{
    print_mem_global(m);
    print_processes(m);
    return 0;
}

static int continfo_open(struct inode *inode, struct file *file)
{
    return single_open(file, continfo_show, NULL);
}

#if LINUX_VERSION_CODE >= KERNEL_VERSION(5, 6, 0)
static const struct proc_ops continfo_fops = {
    .proc_open    = continfo_open,
    .proc_read    = seq_read,
    .proc_lseek   = seq_lseek,
    .proc_release = single_release,
};
#else
static const struct file_operations continfo_fops = {
    .owner   = THIS_MODULE,
    .open    = continfo_open,
    .read    = seq_read,
    .llseek  = seq_lseek,
    .release = single_release,
};
#endif

static struct proc_dir_entry *continfo_entry;

static int __init continfo_init(void)
{
    continfo_entry = proc_create(PROC_NAME, 0444, NULL, &continfo_fops);
    if (!continfo_entry) {
        printk(KERN_ERR "continfo: no se pudo crear /proc/%s\n", PROC_NAME);
        return -ENOMEM;
    }
    printk(KERN_INFO "continfo: modulo cargado, /proc/%s disponible\n", PROC_NAME);
    return 0;
}

static void __exit continfo_exit(void)
{
    proc_remove(continfo_entry);
    printk(KERN_INFO "continfo: modulo descargado, /proc/%s eliminado\n", PROC_NAME);
}

module_init(continfo_init);
module_exit(continfo_exit);

MODULE_LICENSE("GPL");
MODULE_AUTHOR("Alberto");
MODULE_DESCRIPTION("Proyecto 2 SO1 - expone info de memoria y procesos via /proc");
