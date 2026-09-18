package decision

import (
	"sort"

	"proyecto2-so1/daemon/internal/containers"
	"proyecto2-so1/daemon/internal/procinfo"
)

// Limites que el proyecto pide mantener en todo momento.
const (
	MaxLowConsumo = 3
	MaxAlto       = 2
)

func Decide(all []containers.Container, snap *procinfo.Snapshot) []containers.Container {
	applySnapshotMetrics(all, snap)

	var low, alto, intrusos []containers.Container

	for _, c := range all {
		switch c.Tipo {
		case "low-consumo":
			low = append(low, c)
		case "alto-ram", "alto-cpu":
			alto = append(alto, c)
		case "intruso":
			intrusos = append(intrusos, c)
		}
	}

	var toKill []containers.Container
	toKill = append(toKill, excessByConsumption(low, MaxLowConsumo)...)
	toKill = append(toKill, excessByConsumption(alto, MaxAlto)...)
	toKill = append(toKill, intrusos...) // se eliminan todos, sin excepcion

	return toKill
}

// applySnapshotMetrics completa RSS/VSZ/CPU/MEM de cada contenedor cruzando su
// PID con los procesos del snapshot. Sirve de respaldo: main suele poblarlos
// antes de llamar a Decide.
func applySnapshotMetrics(all []containers.Container, snap *procinfo.Snapshot) {
	if snap == nil {
		return
	}

	byPID := make(map[int]procinfo.ProcEntry, len(snap.Procs))
	for _, p := range snap.Procs {
		byPID[p.PID] = p
	}

	for i := range all {
		if all[i].PID == 0 {
			continue
		}

		p, ok := byPID[all[i].PID]
		if !ok {
			continue
		}

		all[i].RSSKB = p.RSSKB
		all[i].VSZKB = p.VSZKB
		all[i].CPUPercent = p.CPUPercent
		all[i].MemPercent = p.MemPercent
	}
}

// excessByConsumption ordena por RSS descendente (mayor consumo primero) y
// devuelve los que sobran, de modo que solo queden los `max` de menor consumo.
func excessByConsumption(list []containers.Container, max int) []containers.Container {
	if len(list) <= max {
		return nil
	}

	sorted := make([]containers.Container, len(list))
	copy(sorted, list)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].RSSKB > sorted[j].RSSKB
	})

	return sorted[:len(sorted)-max]
}
