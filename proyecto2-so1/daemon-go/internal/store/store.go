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
	Timestamp     time.Time `json:"timestamp"`
	RAMTotalKB    uint64    `json:"ram_total_kb"`
	RAMUsadaKB    uint64    `json:"ram_usada_kb"`
	CantProcesos  int       `json:"cant_procesos"`
	CantContenedores int    `json:"cant_contenedores"`
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
	pipe.LPush(s.ctx, "log:snapshots_memoria", data)
	pipe.LTrim(s.ctx, "log:snapshots_memoria", 0, maxHistoryEntries-1)
	_, err = pipe.Exec(s.ctx)
	if err != nil {
		return fmt.Errorf("no se pudo guardar el snapshot en Valkey: %w", err)
	}
	return nil
}