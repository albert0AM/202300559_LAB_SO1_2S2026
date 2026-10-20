package main

import (
	"bytes"
	"io"
	"log"
	"net/http"
	"os"
	"time"
)

var backendURL string

func handleReport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "bad body", http.StatusBadRequest)
		return
	}
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Post(backendURL, "application/json", bytes.NewReader(body))
	if err != nil {
		log.Printf("error llamando al container grpc-client: %v", err)
		http.Error(w, "backend error", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
}

func main() {
	backendURL = os.Getenv("GRPC_CLIENT_ADDR")
	if backendURL == "" {
		backendURL = "http://localhost:8081/report"
	}
	http.HandleFunc("/report", handleReport)
	http.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) { w.Write([]byte("ok")) })
	log.Println("go-api-rest escuchando en :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
