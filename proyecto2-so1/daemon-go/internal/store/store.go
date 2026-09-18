package store

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	redis "github.com/redis/go-redis/v9"

	"proyecto2-so1/daemon/internal/containers"
	"proyecto2-so1/daemon/internal/procinfo"
)

const maxHistoryEntries = 1000

const (
	keySnapshots       = "log:snapshots_memoria"
	keyKills           = "log:eventos_eliminados"
	keyContainerMetric = "log:metricas_contenedores"
)


type Store struct {
	rdb *redis.Client
	ctx context.Context
}

func Connect(addr string) (*Store, error) {
	rdb := redis.NewClient(&redis.Options{Addr: addr})

	ctx := context.Background()
	if err := rdb.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("no se pudo conectar a Valkey en %s: %w", addr, err)
	}

	return &Store{rdb: rdb, ctx: ctx}, nil
}

func (s *Store) Close() error {
	return s.rdb.Close()
}

type killEvent struct {
	Nombre      string    `json:"nombre"`
	Tipo        string    `json:"tipo"`
	Timestamp   time.Time `json:"timestamp"`
	Confirmado  bool      `json:"confirmado"`
}

func (s *Store) SaveKillEvent(c containers.Container, confirmado bool) error {
	ev := killEvent{
		Nombre:     c.Name,
		Tipo:       c.Tipo,
		Timestamp:  time.Now(),
		Confirmado: confirmado,
	}

	data, err := json.Marshal(ev)
	if err != nil {
		return fmt.Errorf("no se pudo serializar el evento: %w", err)
	}

	pipe := s.rdb.TxPipeline()
	pipe.LPush(s.ctx, "log:eventos_eliminados", data)
	pipe.LTrim(s.ctx, "log:eventos_eliminados", 0, maxHistoryEntries-1)
	pipe.HIncrBy(s.ctx, "stats:eliminados_por_tipo", c.Tipo, 1)
	_, err = pipe.Exec(s.ctx)
	if err != nil {
		return fmt.Errorf("no se pudo guardar el evento en Valkey: %w", err)
	}
	return nil
}

type memSnapshot struct {
	Timestamp        time.Time `json:"timestamp"`
	RAMTotalKB       uint64    `json:"ram_total_kb"`
	RAMUsadaKB       uint64    `json:"ram_usada_kb"`
	CantProcesos     int       `json:"cant_procesos"`
	CantContenedores int       `json:"cant_contenedores"`
}

type containerMetric struct {
	Timestamp time.Time `json:"timestamp"`
	ID        string    `json:"id"`
	Nombre    string    `json:"nombre"`
	Tipo      string    `json:"tipo"`
	PID       int       `json:"pid"`
	VSZKB     uint64    `json:"vsz_kb"`
	RSSKB     uint64    `json:"rss_kb"`
	CPUPct    float64   `json:"cpu_pct"`
	MemPct    float64   `json:"mem_pct"`
}

func (s *Store) SaveSnapshot(mem procinfo.MemInfo, cantProcesos int, cantContenedores int) error {
	snap := memSnapshot{
		Timestamp:        time.Now(),
		RAMTotalKB:       mem.TotalKB,
		RAMUsadaKB:       mem.UsedKB,
		CantProcesos:     cantProcesos,
		CantContenedores: cantContenedores,
	}

	data, err := json.Marshal(snap)
	if err != nil {
		return fmt.Errorf("no se pudo serializar el snapshot: %w", err)
	}

	pipe := s.rdb.TxPipeline()
	pipe.LPush(s.ctx, keySnapshots, data)
	pipe.LTrim(s.ctx, keySnapshots, 0, maxHistoryEntries-1)
	_, err = pipe.Exec(s.ctx)
	if err != nil {
		return fmt.Errorf("no se pudo guardar el snapshot en Valkey: %w", err)
	}
	return nil
}

func (s *Store) SaveContainerMetrics(procs []containers.Container) error {
	if len(procs) == 0 {
		return nil
	}

	now := time.Now()
	pipe := s.rdb.TxPipeline()
	for _, c := range procs {
		m := containerMetric{
			Timestamp: now,
			ID:        c.ID,
			Nombre:    c.Name,
			Tipo:      c.Tipo,
			PID:       c.PID,
			VSZKB:     c.VSZKB,
			RSSKB:     c.RSSKB,
			CPUPct:    c.CPUPercent,
			MemPct:    c.MemPercent,
		}
		data, err := json.Marshal(m)
		if err != nil {
			continue
		}
		pipe.LPush(s.ctx, keyContainerMetric, data)
	}
	pipe.LTrim(s.ctx, keyContainerMetric, 0, maxHistoryEntries-1)
	if _, err := pipe.Exec(s.ctx); err != nil {
		return fmt.Errorf("no se pudo guardar las metricas en Valkey: %w", err)
	}
	return nil
}

func (s *Store) GetSnapshots() ([]memSnapshot, error) {
	vals, err := s.rdb.LRange(s.ctx, keySnapshots, 0, -1).Result()
	if err != nil {
		return nil, fmt.Errorf("LRANGE snapshots fallo: %w", err)
	}

	result := make([]memSnapshot, 0, len(vals))
	for i := len(vals) - 1; i >= 0; i-- {
		var m memSnapshot
		if err := json.Unmarshal([]byte(vals[i]), &m); err != nil {
			continue
		}
		result = append(result, m)
	}
	return result, nil
}

func (s *Store) GetKills() ([]killEvent, error) {
	vals, err := s.rdb.LRange(s.ctx, keyKills, 0, -1).Result()
	if err != nil {
		return nil, fmt.Errorf("LRANGE kills fallo: %w", err)
	}

	result := make([]killEvent, 0, len(vals))
	for i := len(vals) - 1; i >= 0; i-- {
		var k killEvent
		if err := json.Unmarshal([]byte(vals[i]), &k); err != nil {
			continue
		}
		result = append(result, k)
	}
	return result, nil
}

func (s *Store) GetContainerMetrics() ([]containerMetric, error) {
	vals, err := s.rdb.LRange(s.ctx, keyContainerMetric, 0, -1).Result()
	if err != nil {
		return nil, fmt.Errorf("LRANGE metricas contenedores fallo: %w", err)
	}

	result := make([]containerMetric, 0, len(vals))
	for i := len(vals) - 1; i >= 0; i-- {
		var m containerMetric
		if err := json.Unmarshal([]byte(vals[i]), &m); err != nil {
			continue
		}
		result = append(result, m)
	}
	return result, nil
}