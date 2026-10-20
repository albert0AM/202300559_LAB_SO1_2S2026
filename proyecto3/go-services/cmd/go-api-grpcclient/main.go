package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	pb "mumn/proto"
)

type report struct {
	Country         string `json:"country"`
	WarplanesInAir  int32  `json:"warplanes_in_air"`
	WarshipsInWater int32  `json:"warships_in_water"`
	Timestamp       string `json:"timestamp"`
}

var client pb.WarReportServiceClient

func handleReport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var rep report
	if err := json.NewDecoder(r.Body).Decode(&rep); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}
	country := pb.Countries(pb.Countries_value[strings.ToLower(rep.Country)])
	if country == pb.Countries_countries_unknown {
		http.Error(w, "country not accepted", http.StatusBadRequest)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	resp, err := client.SendReport(ctx, &pb.WarReportRequest{
		Country:         country,
		WarplanesInAir:  rep.WarplanesInAir,
		WarshipsInWater: rep.WarshipsInWater,
		Timestamp:       rep.Timestamp,
	})
	if err != nil {
		log.Printf("error llamando gRPC: %v", err)
		http.Error(w, "grpc error", http.StatusBadGateway)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": resp.Status})
}

func main() {
	addr := os.Getenv("GRPC_SERVER_ADDR")
	if addr == "" {
		addr = "grpc-server-svc:50051"
	}
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()
	client = pb.NewWarReportServiceClient(conn)

	http.HandleFunc("/report", handleReport)
	http.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) { w.Write([]byte("ok")) })
	log.Println("go-api-grpcclient escuchando en :8081")
	log.Fatal(http.ListenAndServe(":8081", nil))
}
