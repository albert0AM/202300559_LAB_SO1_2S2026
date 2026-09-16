package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"proyecto2-so1/daemon/internal/containers"
	"proyecto2-so1/daemon/internal/decision"
	"proyecto2-so1/daemon/internal/procinfo"
	"proyecto2-so1/daemon/internal/store"
)


const CARNET = "202300559"
const PollInterval = 20 * time.Second
const ValkeyAddr = "localhost:6379"


var dataStore *store.Store

func main() {
	log.Println("=== Daemon Proyecto 2 SO1 iniciando ===")

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := setup(); err != nil {
		log.Fatalf("Error en el setup inicial: %v", err)
	}

	defer cleanup()

	runLoop(ctx)

	log.Println("=== Daemon detenido limpiamente ===")
}

func setup() error {
	log.Println("[setup] Levantando Grafana + Valkey (docker compose)...")
	// TODO: exec.Command("docker", "compose", "-f", "docker/docker-compose.yml", "up", "-d").Run()

	log.Println("[setup] Instalando cronjob de carga de trabajo...")
	// TODO: instalar linea en crontab (*/2 * * * * /ruta/script.sh)

	log.Println("[setup] Cargando modulo de kernel (insmod)...")
	// TODO: exec.Command("sudo", "insmod", "kernel/continfo.ko").Run()

	log.Printf("[setup] Conectando a Valkey en %s...\n", ValkeyAddr)
	s, err := store.Connect(ValkeyAddr)
	if err != nil {
		log.Printf("[setup] ADVERTENCIA: no se pudo conectar a Valkey (%v). Se sigue sin guardar historial.", err)
	} else {
		dataStore = s
		log.Println("[setup] Conectado a Valkey correctamente.")
	}

	log.Printf("[setup] Listo. Leyendo desde %s cada %s\n", procinfo.ProcPath(CARNET), PollInterval)
	return nil
}

func cleanup() {
	log.Println("[cleanup] Quitando cronjob...")
	// TODO: quitar la linea del crontab

	log.Println("[cleanup] Descargando modulo de kernel (rmmod)...")
	// TODO: exec.Command("sudo", "rmmod", "continfo").Run()

	if dataStore != nil {
		log.Println("[cleanup] Cerrando conexion a Valkey...")
		if err := dataStore.Close(); err != nil {
			log.Printf("[cleanup] Error cerrando Valkey: %v", err)
		}
	}

	log.Println("[cleanup] Listo.")
}


func runLoop(ctx context.Context) {
	ticker := time.NewTicker(PollInterval)
	defer ticker.Stop()

	path := procinfo.ProcPath(CARNET)

	var prevSnap *procinfo.Snapshot
	var prevTime time.Time

	for {
		select {
		case <-ctx.Done():
			log.Println("[loop] Señal de apagado recibida, saliendo del loop...")
			return

		case now := <-ticker.C:
			snap, err := procinfo.ReadSnapshot(path)
			if err != nil {
				log.Printf("[loop] Error leyendo snapshot: %v", err)
				continue
			}

			log.Printf("[loop] Lectura OK: RAM usada %d/%d kB, %d procesos",
				snap.Mem.UsedKB, snap.Mem.TotalKB, len(snap.Procs))

			if prevSnap != nil {
				elapsed := now.Sub(prevTime).Seconds()
				printTopByCPU(snap, prevSnap, elapsed)
			}

			cantContenedores := enforceContainerLimits()

			if dataStore != nil {
				if err := dataStore.SaveSnapshot(snap.Mem, len(snap.Procs), cantContenedores); err != nil {
					log.Printf("[valkey] Error guardando snapshot: %v", err)
				}
			}

			prevSnap = snap
			prevTime = now
		}
	}
}

func enforceContainerLimits() int {
	all, err := containers.ListManaged()
	if err != nil {
		log.Printf("[decision] Error listando contenedores: %v", err)
		return 0
	}

	toKill := decision.Decide(all)
	if len(toKill) == 0 {
		log.Printf("[decision] %d contenedores del proyecto corriendo, nada que matar", len(all))
		return len(all)
	}

	log.Printf("[decision] %d contenedores corriendo, matando %d para mantener el balance", len(all), len(toKill))
	for _, c := range toKill {
		if err := containers.Kill(c.ID); err != nil {
			log.Printf("    [ERROR] no se pudo matar %s (%s): %v", c.Name, c.Tipo, err)
			continue
		}
		log.Printf("    [MATADO] %s (tipo=%s, creado=%s)", c.Name, c.Tipo, c.CreatedAt.Format(time.RFC3339))

		if dataStore != nil {
			if err := dataStore.SaveKillEvent(c); err != nil {
				log.Printf("    [valkey] Error guardando evento: %v", err)
			}
		}
	}

	return len(all)
}

func printTopByCPU(curr, prev *procinfo.Snapshot, elapsed float64) {
	prevByPID := make(map[int]procinfo.ProcEntry, len(prev.Procs))
	for _, p := range prev.Procs {
		prevByPID[p.PID] = p
	}

	log.Println("[loop] --- Top procesos por %CPU (desde la lectura anterior) ---")
	count := 0
	for _, p := range curr.Procs {
		old, ok := prevByPID[p.PID]
		if !ok {
			continue // proceso nuevo, no tenemos punto de comparacion
		}
		cpuPct := procinfo.CPUPercent(old.UTime, old.STime, p.UTime, p.STime, elapsed)
		if cpuPct > 0.01 {
			log.Printf("    PID %d (%s): %.2f%% CPU, %.2f%% MEM", p.PID, p.Name, cpuPct, p.MemPercent)
			count++
		}
		if count >= 5 {
			break
		}
	}
}