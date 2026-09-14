obj-m += continfo.o

# El modulo final se llama continfo.ko, pero se arma con varios .o.
# continfo-objs le dice al sistema de build del kernel que enlace
# estos tres archivos objeto en un solo modulo.
continfo-objs := continfo_main.o mem_info.o proc_info.o

KDIR := /lib/modules/$(shell uname -r)/build
PWD  := $(shell pwd)

all:
	$(MAKE) -C $(KDIR) M=$(PWD) modules

clean:
	$(MAKE) -C $(KDIR) M=$(PWD) clean

load:
	sudo insmod continfo.ko

unload:
	sudo rmmod continfo

show:
	cat /proc/continfo_pr2_so1_*