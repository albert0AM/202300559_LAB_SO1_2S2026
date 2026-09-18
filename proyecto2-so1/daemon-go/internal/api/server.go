package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"time"

	"proyecto2-so1/daemon/internal/store"
)

const listenAddr = "0.0.0.0:8080"

type Server struct {
	store   *store.Store
	httpSrv *http.Server
}

type estadoResponse struct {
	RAMTotal         float64   `json:"ram_total"`
	RAMUsada         float64   `json:"ram_usada"`
	RAMLibre         float64   `json:"ram_libre"`
	UltimaLectura    time.Time `json:"ultima_lectura"`
	CantProcesos     int       `json:"cant_procesos"`
	CantContenedores int       `json:"cant_contenedores"`
	EliminadosTotal  int       `json:"eliminados_totales"`
	ConfirmadosEBPF  int       `json:"confirmados_ebpf"`
}

type snapPoint struct {
	Time             int64   `json:"time"`
	RAMTotal         float64 `json:"ram_total"`
	RAMUsada         float64 `json:"ram_usada"`
	RAMLibre         float64 `json:"ram_libre"`
	CantProcesos     int     `json:"cant_procesos"`
	CantContenedores int     `json:"cant_contenedores"`
}

type killPoint struct {
	Time       int64  `json:"time"`
	Nombre     string `json:"nombre"`
	Tipo       string `json:"tipo"`
	Confirmado bool   `json:"confirmado"`
}

type killsBucket struct {
	Time       int64 `json:"time"`
	Eliminados int   `json:"eliminados"`
}

type rankItem struct {
	Nombre string  `json:"nombre"`
	Tipo   string  `json:"tipo"`
	PID    int     `json:"pid"`
	Valor  float64 `json:"valor"`
}

func New(s *store.Store) *Server {
	mux := http.NewServeMux()
	srv := &Server{store: s}

	mux.HandleFunc("/api/estado", srv.handleEstado)
	mux.HandleFunc("/api/snapshots", srv.handleSnapshots)
	mux.HandleFunc("/api/kills", srv.handleKills)
	mux.HandleFunc("/api/kills/series", srv.handleKillsSeries)
	mux.HandleFunc("/api/ranking/ram", srv.handleRankRAM)
	mux.HandleFunc("/api/ranking/cpu", srv.handleRankCPU)
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	srv.httpSrv = &http.Server{Addr: listenAddr, Handler: mux}
	return srv
}

func (s *Server) Start() error {
	go func() {
		if err := s.httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			fmt.Printf("[api] error en el servidor HTTP: %v\n", err)
		}
	}()
	return nil
}

func (s *Server) Shutdown() error {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	return s.httpSrv.Shutdown(ctx)
}

func (s *Server) handleEstado(w http.ResponseWriter, r *http.Request) {
	if s.store == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "sin conexion a Valkey"})
		return
	}

	snaps, err := s.store.GetSnapshots()
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}
	kills, err := s.store.GetKills()
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}

	resp := estadoResponse{}
	if len(snaps) > 0 {
		last := snaps[len(snaps)-1]
		resp.RAMTotal = toBytes(last.RAMTotalKB)
		resp.RAMUsada = toBytes(last.RAMUsadaKB)
		resp.RAMLibre = toBytes(SubKBs(last.RAMTotalKB, last.RAMUsadaKB))
		resp.UltimaLectura = last.Timestamp
		resp.CantProcesos = last.CantProcesos
		resp.CantContenedores = last.CantContenedores
	}

	for _, k := range kills {
		resp.EliminadosTotal++
		if k.Confirmado {
			resp.ConfirmadosEBPF++
		}
	}

	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleSnapshots(w http.ResponseWriter, r *http.Request) {
	if s.store == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "sin conexion a Valkey"})
		return
	}

	snaps, err := s.store.GetSnapshots()
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}

	points := make([]snapPoint, 0, len(snaps))
	for _, m := range snaps {
		points = append(points, snapPoint{
			Time:             m.Timestamp.UnixMilli(),
			RAMTotal:         toBytes(m.RAMTotalKB),
			RAMUsada:         toBytes(m.RAMUsadaKB),
			RAMLibre:         toBytes(SubKBs(m.RAMTotalKB, m.RAMUsadaKB)),
			CantProcesos:     m.CantProcesos,
			CantContenedores: m.CantContenedores,
		})
	}
	writeJSON(w, http.StatusOK, points)
}

func (s *Server) handleKills(w http.ResponseWriter, r *http.Request) {
	if s.store == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "sin conexion a Valkey"})
		return
	}

	kills, err := s.store.GetKills()
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}

	filterConfirmados := r.URL.Query().Get("confirmados")
	points := make([]killPoint, 0, len(kills))
	for _, k := range kills {
		if filterConfirmados == "true" && !k.Confirmado {
			continue
		}
		points = append(points, killPoint{
			Time:       k.Timestamp.UnixMilli(),
			Nombre:     k.Nombre,
			Tipo:       k.Tipo,
			Confirmado: k.Confirmado,
		})
	}
	writeJSON(w, http.StatusOK, points)
}

func (s *Server) handleKillsSeries(w http.ResponseWriter, r *http.Request) {
	if s.store == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "sin conexion a Valkey"})
		return
	}

	kills, err := s.store.GetKills()
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}

	bucketSec := 60
	if raw := r.URL.Query().Get("bucket"); raw != "" {
		if v, err := strconv.Atoi(raw); err == nil && v > 0 {
			bucketSec = v
		}
	}

	if len(kills) == 0 {
		writeJSON(w, http.StatusOK, []killsBucket{})
		return
	}

	sort.Slice(kills, func(i, j int) bool { return kills[i].Timestamp.Before(kills[j].Timestamp) })

	buckets := make(map[int64]int)
	minMs := kills[0].Timestamp.UnixMilli()
	maxMs := kills[len(kills)-1].Timestamp.UnixMilli()
	bucketMs := int64(bucketSec) * 1000

	for _, k := range kills {
		idx := k.Timestamp.UnixMilli() / bucketMs
		buckets[idx]++
	}

	var result []killsBucket
	total := int((maxMs-minMs)/bucketMs + 1)
	if total > 500 {
		total = 500
	}
	for i := 0; i < total; i++ {
		idx := minMs/bucketMs + int64(i)
		result = append(result, killsBucket{
			Time:       idx * bucketMs,
			Eliminados: buckets[idx],
		})
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) handleRankRAM(w http.ResponseWriter, r *http.Request) {
	s.handleRank(w, true)
}

func (s *Server) handleRankCPU(w http.ResponseWriter, r *http.Request) {
	s.handleRank(w, false)
}

func (s *Server) handleRank(w http.ResponseWriter, byRAM bool) {
	if s.store == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "sin conexion a Valkey"})
		return
	}

	metrics, err := s.store.GetContainerMetrics()
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}

	best := make(map[string]*rankBest)
	for _, m := range metrics {
		cur, ok := best[m.ID]
		if !ok {
			cur = &rankBest{ID: m.ID, Nombre: m.Nombre, Tipo: m.Tipo, PID: m.PID}
			best[m.ID] = cur
		}
		if byRAM && m.RSSKB > cur.RSSKB {
			cur.RSSKB = m.RSSKB
			cur.Nombre = m.Nombre
			cur.Tipo = m.Tipo
			cur.PID = m.PID
		}
		if !byRAM && m.CPUPct > cur.CPUPct {
			cur.CPUPct = m.CPUPct
			cur.Nombre = m.Nombre
			cur.Tipo = m.Tipo
			cur.PID = m.PID
		}
	}

	ranks := make([]rankItem, 0, len(best))
	for _, c := range best {
		val := float64(c.RSSKB)
		if !byRAM {
			val = c.CPUPct
		}
		if byRAM {
			val = toBytes(c.RSSKB)
		}
		ranks = append(ranks, rankItem{
			Nombre: c.Nombre,
			Tipo:   c.Tipo,
			PID:    c.PID,
			Valor:  val,
		})
	}

	sort.Slice(ranks, func(i, j int) bool { return ranks[i].Valor > ranks[j].Valor })
	if len(ranks) > 5 {
		ranks = ranks[:5]
	}

	writeJSON(w, http.StatusOK, ranks)
}

type rankBest struct {
	ID     string
	Nombre string
	Tipo   string
	PID    int
	RSSKB  uint64
	CPUPct float64
}

func toBytes(kb uint64) float64 {
	return float64(kb) * 1024
}

func SubKBs(a, b uint64) uint64 {
	if a <= b {
		return 0
	}
	return a - b
}

func writeJSON(w http.ResponseWriter, status int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}