package main

import (
	"bytes"
	"context"
	"encoding/json"
	"log"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"google.golang.org/grpc"

	pb "mumn/proto"
)

type server struct {
	pb.UnimplementedWarReportServiceServer
	writerURL string
	client    *http.Client
}

func (s *server) SendReport(ctx context.Context, r *pb.WarReportRequest) (*pb.WarReportResponse, error) {
	msg := map[string]interface{}{
		"country":           strings.ToUpper(r.Country.String()),
		"warplanes_in_air":  r.WarplanesInAir,
		"warships_in_water": r.WarshipsInWater,
		"timestamp":         r.Timestamp,
	}
	body, _ := json.Marshal(msg)
	resp, err := s.client.Post(s.writerURL, "application/json", bytes.NewReader(body))
	if err != nil {
		log.Printf("error llamando al writer: %v", err)
		return &pb.WarReportResponse{Status: "error"}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return &pb.WarReportResponse{Status: "error"}, nil
	}
	return &pb.WarReportResponse{Status: "ok"}, nil
}

func main() {
	writerURL := os.Getenv("RABBITMQ_WRITER_ADDR")
	if writerURL == "" {
		writerURL = "http://localhost:8082/publish"
	}
	lis, err := net.Listen("tcp", ":50051")
	if err != nil {
		log.Fatal(err)
	}
	g := grpc.NewServer()
	pb.RegisterWarReportServiceServer(g, &server{writerURL: writerURL, client: &http.Client{Timeout: 5 * time.Second}})
	log.Println("grpc-server escuchando en :50051")
	log.Fatal(g.Serve(lis))
}
