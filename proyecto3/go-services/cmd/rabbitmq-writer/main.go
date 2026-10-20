package main

import (
	"context"
	"io"
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

const queueName = "war_reports"

type writer struct {
	mu   sync.Mutex
	conn *amqp.Connection
	ch   *amqp.Channel
	url  string
}

func (w *writer) connect() error {
	conn, err := amqp.Dial(w.url)
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
	w.conn, w.ch = conn, ch
	return nil
}

func (w *writer) publish(body []byte) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.ch == nil || w.conn.IsClosed() {
		if err := w.connect(); err != nil {
			return err
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return w.ch.PublishWithContext(ctx, "", queueName, false, false, amqp.Publishing{
		ContentType:  "application/json",
		DeliveryMode: amqp.Persistent,
		Body:         body,
	})
}

func main() {
	url := os.Getenv("RABBITMQ_URL")
	if url == "" {
		url = "amqp://guest:guest@rabbitmq:5672/"
	}
	w := &writer{url: url}

	http.HandleFunc("/publish", func(rw http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(rw, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(rw, "bad body", http.StatusBadRequest)
			return
		}
		if err := w.publish(body); err != nil {
			log.Printf("error publicando en RabbitMQ: %v", err)
			http.Error(rw, "publish error", http.StatusBadGateway)
			return
		}
		rw.WriteHeader(http.StatusOK)
	})
	http.HandleFunc("/health", func(rw http.ResponseWriter, _ *http.Request) { rw.Write([]byte("ok")) })

	log.Println("rabbitmq-writer escuchando en :8082")
	log.Fatal(http.ListenAndServe(":8082", nil))
}
