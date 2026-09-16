// Package containers habla con el Docker CLI para listar y matar los
// contenedores que el proyecto 
package containers

import (
	"fmt"
	"os/exec"
	"strings"
	"time"
)

type Container struct {
	ID        string
	Name      string
	Tipo      string // "low-consumo" | "alto-ram" | "alto-cpu" | "intruso"
	CreatedAt time.Time
}

const managedLabelFilter = "label=proyecto2.managed=true"


const dockerCreatedLayout = "2006-01-02 15:04:05 -0700 MST"

func ListManaged() ([]Container, error) {

	format := `{{.ID}}|||{{.Names}}|||{{.Label "proyecto2.tipo"}}|||{{.CreatedAt}}`

	cmd := exec.Command("docker", "ps",
		"--filter", managedLabelFilter,
		"--format", format,
	)
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("docker ps fallo: %w", err)
	}

	trimmed := strings.TrimSpace(string(out))
	if trimmed == "" {
		return nil, nil // no hay contenedores del proyecto corriendo
	}

	var result []Container
	for _, line := range strings.Split(trimmed, "\n") {
		parts := strings.Split(line, "|||")
		if len(parts) < 4 {
			continue 
		}

		created, err := time.Parse(dockerCreatedLayout, parts[3])
		if err != nil {
			created = time.Time{}
		}

		result = append(result, Container{
			ID:        parts[0],
			Name:      parts[1],
			Tipo:      parts[2],
			CreatedAt: created,
		})
	}
	return result, nil
}

// Kill mata un contenedor enviando SIGKILL (docker kill). 
func Kill(id string) error {
	cmd := exec.Command("docker", "kill", id)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("docker kill %s fallo: %w (salida: %s)", id, err, strings.TrimSpace(string(out)))
	}
	return nil
}