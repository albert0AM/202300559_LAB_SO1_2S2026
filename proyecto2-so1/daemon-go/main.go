package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"proyecto2-so1/daemon/internal/api"
	"proyecto2-so1/daemon/internal/containers"
	"proyecto2-so1/daemon/internal/decision"
	"proyecto2-so1/daemon/internal/ebpfkill"
	"proyecto2-so1/daemon/internal/procinfo"
	"proyecto2-so1/daemon/internal/store"
)

const CARNET = "202300559"

const PollInterval = 20 * time.Second

const ValkeyAddr = "localhost:6379"

var dataStore *store.Store
var ebpfMonitor *ebpfkill.Monitor
var ebpfRegistry *ebpfkill.Registry
var apiServer *api.Server

const KillConfirmTimeout = 3 * time.Second

const (
	kernelModulePath = "/home/alberto/202300559_LAB_SO1_2S2026/proyecto2-so1/kernel/continfo.ko"
	kernelModuleName = "continfo"
	dockerComposeDir = "/home/alberto/202300559_LAB_SO1_2S2026/proyecto2-so1/docker"
	cronScriptPath   = "/home/alberto/202300559_LAB_SO1_2S2026/proyecto2-so1/scripts/launch_containers.sh"
	cronLogPath      = "/home/alberto/202300559_LAB_SO1_2S2026/proyecto2-so1/scripts/cron.log"
)

var cronLine = fmt.Sprintf(
	"*/2 * * * * PATH=/usr/bin:/bin %s >> %s 2>&1",
	cronScriptPath, cronLogPath,
)

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
	if err := startDockerCompose(); err != nil {
		return fmt.Errorf("[setup] error en docker compose: %w", err)
	}

	log.Println("[setup] Instalando cronjob de carga de trabajo...")
	if err := installCronjob(); err != nil {
		return fmt.Errorf("[setup] error instalando cronjob: %w", err)
	}

	log.Println("[setup] Cargando modulo de kernel (insmod)...")
	if err := loadKernelModule(); err != nil {
		return fmt.Errorf("[setup] error cargando modulo kernel: %w", err)
	}

	log.Println("[setup] Cargando programa eBPF (sys_enter_kill)...")
	mon, err := ebpfkill.Start()

	if err != nil {
		log.Printf("[setup] ADVERTENCIA: no se pudo cargar eBPF (%v). ¿Estas corriendo como root (sudo)? Se sigue sin confirmacion de kernel.", err)
	} else {
		ebpfMonitor = mon
		ebpfRegistry = ebpfkill.NewRegistry()
		go func() {
			for ev := range ebpfMonitor.Events() {
				ebpfRegistry.Dispatch(ev)
			}
		}()
		log.Println("[setup] Programa eBPF cargado y enganchado correctamente.")
	}

	log.Printf("[setup] Conectando a Valkey en %s...\n", ValkeyAddr)
	s, err := store.Connect(ValkeyAddr)
	if err != nil {
		log.Printf("[setup] ADVERTENCIA: no se pudo conectar a Valkey (%v). Se sigue sin guardar historial.", err)
	} else {
		dataStore = s
		log.Println("[setup] Conectado a Valkey correctamente.")
	}

	apiServer = api.New(dataStore)
	if err := apiServer.Start(); err != nil {
		return fmt.Errorf("[setup] no se pudo iniciar el API server: %w", err)
	}
	log.Println("[setup] API HTTP escuchando en :8080 para Grafana.")

	log.Printf("[setup] Listo. Leyendo desde %s cada %s\n", procinfo.ProcPath(CARNET), PollInterval)
	return nil
}

func cleanup() {
	log.Println("[cleanup] Quitando cronjob...")
	if err := removeCronjob(); err != nil {
		log.Printf("[cleanup] ADVERTENCIA: no se pudo quitar cronjob: %v", err)
	}

	log.Println("[cleanup] Descargando modulo de kernel (rmmod)...")
	if err := unloadKernelModule(); err != nil {
		log.Printf("[cleanup] ADVERTENCIA: no se pudo descargar modulo: %v", err)
	}

	if ebpfMonitor != nil {
		log.Println("[cleanup] Descargando programa eBPF...")
		ebpfMonitor.Close()
	}

	if dataStore != nil {
		log.Println("[cleanup] Cerrando conexion a Valkey...")
		if err := dataStore.Close(); err != nil {
			log.Printf("[cleanup] Error cerrando Valkey: %v", err)
		}
	}

	if apiServer != nil {
		log.Println("[cleanup] Deteniendo API HTTP...")
		if err := apiServer.Shutdown(); err != nil {
			log.Printf("[cleanup] Error deteniendo API: %v", err)
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

			cantContenedores := enforceContainerLimits(snap)

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

func enforceContainerLimits(snap *procinfo.Snapshot) int {
	all, err := containers.ListManaged()
	if err != nil {
		log.Printf("[decision] Error listando contenedores: %v", err)
		return 0
	}

	procsByPID := make(map[int]procinfo.ProcEntry, len(snap.Procs))
	for _, p := range snap.Procs {
		procsByPID[p.PID] = p
	}

	for i := range all {
		pid, err := containers.GetPID(all[i].ID)
		if err != nil {
			log.Printf("[decision] no se pudo obtener el PID de %s (%s): %v", all[i].Name, all[i].Tipo, err)
			continue
		}
		all[i].PID = pid

		if p, ok := procsByPID[pid]; ok {
			all[i].RSSKB = p.RSSKB
			all[i].VSZKB = p.VSZKB
			all[i].CPUPercent = p.CPUPercent
			all[i].MemPercent = p.MemPercent
		}
	}

	if dataStore != nil {
		if err := dataStore.SaveContainerMetrics(all); err != nil {
			log.Printf("[valkey] Error guardando metricas de contenedores: %v", err)
		}
	}

	toKill := decision.Decide(all, snap)
	if len(toKill) == 0 {
		log.Printf("[decision] %d contenedores del proyecto corriendo, nada que matar", len(all))
		return len(all)
	}

	log.Printf("[decision] %d contenedores corriendo, matando %d para mantener el balance", len(all), len(toKill))
	for _, c := range toKill {
		pid := c.PID
		var pidErr error
		if pid == 0 {
			pid, pidErr = containers.GetPID(c.ID)
		}

		if pidErr != nil {
			log.Printf("    [ERROR] no se pudo obtener el PID de %s (%s): %v", c.Name, c.Tipo, pidErr)
			if err := containers.Kill(c.ID); err != nil {
				log.Printf("    [ERROR] no se pudo matar %s (%s): %v", c.Name, c.Tipo, err)
				continue
			}
			log.Printf("    [MATADO] %s (tipo=%s) - via docker kill, sin verificacion eBPF", c.Name, c.Tipo)
			if dataStore != nil {
				if err := dataStore.SaveKillEvent(c, false); err != nil {
					log.Printf("    [valkey] Error guardando evento: %v", err)
				}
			}
			continue
		}

		var evCh <-chan ebpfkill.Event
		canConfirm := ebpfRegistry != nil
		if canConfirm {
			evCh = ebpfRegistry.Register(int64(pid))
		}

		if err := containers.Kill(c.ID); err != nil {
			log.Printf("    [ERROR] no se pudo matar %s (%s): %v", c.Name, c.Tipo, err)
			if canConfirm {
				ebpfRegistry.Unregister(int64(pid))
			}
			continue
		}

		confirmado := false
		if canConfirm {
			if _, ok := ebpfkill.Wait(int64(pid), evCh, ebpfRegistry, KillConfirmTimeout); ok {
				confirmado = true
				log.Printf("    [MATADO] %s (tipo=%s) - confirmado por eBPF (PID %d)", c.Name, c.Tipo, pid)
			} else {
				log.Printf("    [MATADO] %s (tipo=%s) - eBPF NO confirmo dentro de %s (PID %d)", c.Name, c.Tipo, KillConfirmTimeout, pid)
			}
		} else {
			log.Printf("    [MATADO] %s (tipo=%s, creado=%s) - sin verificacion eBPF disponible", c.Name, c.Tipo, c.CreatedAt.Format(time.RFC3339))
		}

		if dataStore != nil {
			if err := dataStore.SaveKillEvent(c, confirmado); err != nil {
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
			continue
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

func startDockerCompose() error {
	cmd := exec.Command("docker", "compose", "up", "-d")
	cmd.Dir = dockerComposeDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("docker compose up fallo: %v (salida: %s)", err, string(out))
	}
	return nil
}

func loadKernelModule() error {
	cmd := exec.Command("insmod", kernelModulePath)
	out, err := cmd.CombinedOutput()
	if err != nil {
		if strings.Contains(string(out), "File exists") {
			log.Println("[setup] Modulo ya estaba cargado, se continua.")
			return nil
		}
		return fmt.Errorf("insmod fallo: %v (salida: %s)", err, string(out))
	}
	return nil
}

func unloadKernelModule() error {
	cmd := exec.Command("rmmod", kernelModuleName)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("rmmod fallo: %v (salida: %s)", err, string(out))
	}
	return nil
}

func installCronjob() error {
	listCmd := exec.Command("crontab", "-l")
	existing, _ := listCmd.Output()

	if strings.Contains(string(existing), cronLine) {
		log.Println("[setup] Cronjob ya estaba instalado, se continua.")
		return nil
	}

	newCrontab := string(existing)
	if newCrontab != "" && !strings.HasSuffix(newCrontab, "\n") {
		newCrontab += "\n"
	}
	newCrontab += cronLine + "\n"

	installCmd := exec.Command("crontab", "-")
	installCmd.Stdin = strings.NewReader(newCrontab)
	out, err := installCmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("crontab (instalar) fallo: %v (salida: %s)", err, string(out))
	}
	return nil
}

func removeCronjob() error {
	listCmd := exec.Command("crontab", "-l")
	existing, err := listCmd.Output()
	if err != nil {
		return nil
	}

	lines := strings.Split(string(existing), "\n")
	var kept []string
	for _, line := range lines {
		if strings.TrimSpace(line) == "" || line == cronLine {
			continue
		}
		kept = append(kept, line)
	}
	newCrontab := strings.Join(kept, "\n")
	if newCrontab != "" {
		newCrontab += "\n"
	}

	installCmd := exec.Command("crontab", "-")
	installCmd.Stdin = strings.NewReader(newCrontab)
	out, err := installCmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("crontab (remover) fallo: %v (salida: %s)", err, string(out))
	}
	return nil
}
