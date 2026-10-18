package main

import (
	"context"
	"encoding/json"
	"log"
	"net"
	"os"
	"strings"
	"sync"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	"google.golang.org/grpc"

	pb "mumn/proto"
)

const queueName = "war_reports"

type server struct {
	pb.UnimplementedWarReportServiceServer
	mu   sync.Mutex
	conn *amqp.Connection
	ch   *amqp.Channel
	url  string
}

func (s *server) connect() error {
	conn, err := amqp.Dial(s.url)
	if err != nil {
		return err
	}
	ch, err := conn.Channel()
	if err != nil {
		conn.Close()
		return err
	}
	if _, err = ch.QueueDeclare(queueName, true, false, false, false, nil); err != nil {
		conn.Close()
		return err
	}
	s.conn, s.ch = conn, ch
	return nil
}

func (s *server) publish(body []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.ch == nil || s.conn.IsClosed() {
		if err := s.connect(); err != nil {
			return err
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return s.ch.PublishWithContext(ctx, "", queueName, false, false, amqp.Publishing{
		ContentType:  "application/json",
		DeliveryMode: amqp.Persistent,
		Body:         body,
	})
}

func (s *server) SendReport(ctx context.Context, r *pb.WarReportRequest) (*pb.WarReportResponse, error) {
	msg := map[string]interface{}{
		"country":           strings.ToUpper(r.Country.String()),
		"warplanes_in_air":  r.WarplanesInAir,
		"warships_in_water": r.WarshipsInWater,
		"timestamp":         r.Timestamp,
	}
	body, _ := json.Marshal(msg)
	if err := s.publish(body); err != nil {
		log.Printf("error publicando en RabbitMQ: %v", err)
		return &pb.WarReportResponse{Status: "error"}, err
	}
	return &pb.WarReportResponse{Status: "ok"}, nil
}

func main() {
	url := os.Getenv("RABBITMQ_URL")
	if url == "" {
		url = "amqp://guest:guest@rabbitmq:5672/"
	}
	lis, err := net.Listen("tcp", ":50051")
	if err != nil {
		log.Fatal(err)
	}
	g := grpc.NewServer()
	pb.RegisterWarReportServiceServer(g, &server{url: url})
	log.Println("grpc-server escuchando en :50051")
	log.Fatal(g.Serve(lis))
}
