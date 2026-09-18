package procinfo

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
)

type MemInfo struct {
	TotalKB uint64
	FreeKB  uint64
	UsedKB  uint64
}

type ProcEntry struct {
	PID        int
	Name       string
	Cmd        string
	VSZKB      uint64
	RSSKB      uint64
	MemPercent float64
	CPUPercent float64
	UTime      uint64
	STime      uint64
}


type Snapshot struct {
	Mem   MemInfo
	Procs []ProcEntry
}


func ProcPath(carnet string) string {
	return "/proc/continfo_pr2_so1_" + carnet
}


func ReadSnapshot(path string) (*Snapshot, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("no se pudo abrir %s: %w", path, err)
	}
	defer f.Close()

	snap := &Snapshot{}
	scanner := bufio.NewScanner(f)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		switch {
		case strings.HasPrefix(line, "RAM Total:"):
			snap.Mem.TotalKB = parseKBValue(line, "RAM Total:")
		case strings.HasPrefix(line, "RAM Libre:"):
			snap.Mem.FreeKB = parseKBValue(line, "RAM Libre:")
		case strings.HasPrefix(line, "RAM Usada:"):
			snap.Mem.UsedKB = parseKBValue(line, "RAM Usada:")
		case strings.HasPrefix(line, "PID:"):
			entry, err := parseProcLine(line)
			if err != nil {
				continue
			}
			snap.Procs = append(snap.Procs, entry)
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error leyendo %s: %w", path, err)
	}

	return snap, nil
}


func parseKBValue(line, prefix string) uint64 {
	rest := strings.TrimPrefix(line, prefix)
	rest = strings.TrimSuffix(strings.TrimSpace(rest), "kB")
	rest = strings.TrimSpace(rest)
	val, _ := strconv.ParseUint(rest, 10, 64)
	return val
}

func parseProcLine(line string) (ProcEntry, error) {
	var entry ProcEntry

	fields := strings.Split(line, "|")
	for _, raw := range fields {
		field := strings.TrimSpace(raw)

		switch {
		case strings.HasPrefix(field, "PID:"):
			v := strings.TrimSpace(strings.TrimPrefix(field, "PID:"))
			pid, err := strconv.Atoi(v)
			if err != nil {
				return entry, fmt.Errorf("PID invalido: %w", err)
			}
			entry.PID = pid

		case strings.HasPrefix(field, "Nombre:"):
			entry.Name = strings.TrimSpace(strings.TrimPrefix(field, "Nombre:"))

		case strings.HasPrefix(field, "Cmd:"):
			entry.Cmd = strings.TrimSpace(strings.TrimPrefix(field, "Cmd:"))

		case strings.HasPrefix(field, "VSZ:"):
			v := strings.TrimSuffix(strings.TrimSpace(strings.TrimPrefix(field, "VSZ:")), "kB")
			entry.VSZKB, _ = strconv.ParseUint(strings.TrimSpace(v), 10, 64)

		case strings.HasPrefix(field, "RSS:"):
			v := strings.TrimSuffix(strings.TrimSpace(strings.TrimPrefix(field, "RSS:")), "kB")
			entry.RSSKB, _ = strconv.ParseUint(strings.TrimSpace(v), 10, 64)

		case strings.HasPrefix(field, "%MEM:"):
			v := strings.TrimSuffix(strings.TrimSpace(strings.TrimPrefix(field, "%MEM:")), "%")
			entry.MemPercent, _ = strconv.ParseFloat(strings.TrimSpace(v), 64)

		case strings.HasPrefix(field, "%CPU:"):
			v := strings.TrimSuffix(strings.TrimSpace(strings.TrimPrefix(field, "%CPU:")), "%")
			entry.CPUPercent, _ = strconv.ParseFloat(strings.TrimSpace(v), 64)

		case strings.HasPrefix(field, "utime:"):
			v := strings.TrimSpace(strings.TrimPrefix(field, "utime:"))
			entry.UTime, _ = strconv.ParseUint(v, 10, 64)

		case strings.HasPrefix(field, "stime:"):
			v := strings.TrimSpace(strings.TrimPrefix(field, "stime:"))
			entry.STime, _ = strconv.ParseUint(v, 10, 64)
		}
	}

	if entry.PID == 0 {
		return entry, fmt.Errorf("linea sin PID valido: %q", line)
	}
	return entry, nil
}