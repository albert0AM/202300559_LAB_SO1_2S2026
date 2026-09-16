package decision

import (
	"sort"

	"proyecto2-so1/daemon/internal/containers"
)

// Limites que el proyecto pide mantener en todo momento.
const (
	MaxLowConsumo = 3 
	MaxAlto       = 2 
)

func Decide(all []containers.Container) []containers.Container {
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
	toKill = append(toKill, excessOldest(low, MaxLowConsumo)...)
	toKill = append(toKill, excessOldest(alto, MaxAlto)...)
	toKill = append(toKill, intrusos...) // se eliminan todos, sin excepcion

	return toKill
}


func excessOldest(list []containers.Container, max int) []containers.Container {
	if len(list) <= max {
		return nil
	}

	sorted := make([]containers.Container, len(list))
	copy(sorted, list)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].CreatedAt.Before(sorted[j].CreatedAt)
	})

	return sorted[:len(sorted)-max]
}