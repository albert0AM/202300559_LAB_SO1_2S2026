package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"
)

const carnet = "202300559"

var (
	apiID   string            // "1", "2" o "3"
	vmID    string            // VM donde corre esta API
	port    string
	peerURL = map[string]string{} 
	peerVM  = map[string]string{} 

type healthResponse struct {
	Status    string `json:"status"`
	Message   string `json:"message"`
	Timestamp string `json:"timestamp"`
	VM        string `json:"VM"`
	Carnet    string `json:"carnet"`
}

type callResponse struct {
	APIName    string `json:"apiname"`
	Message    string `json:"message"`
	Connection bool   `json:"connection"`
	Carnet     string `json:"carnet"`
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	resp := healthResponse{
		Status:    "UP",
		Message:   fmt.Sprintf("API%s is Ready", apiID),
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		VM:        vmID,
		Carnet:    carnet,
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func makeCallHandler(targetID string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		baseURL := peerURL[targetID]
		targetVM := peerVM[targetID]

		connection := false
		message := fmt.Sprintf("ERROR: The API%s located on the %s is not working", targetID, targetVM)

		client := http.Client{Timeout: 3 * time.Second}
		resp, err := client.Get(baseURL + "/health")
		if err == nil {
			defer resp.Body.Close()
			var body healthResponse
			if json.NewDecoder(resp.Body).Decode(&body) == nil && body.Status == "UP" {
				connection = true
				message = fmt.Sprintf("The API%s located on the %s is working", targetID, targetVM)
			}
		}

		out := callResponse{
			APIName:    "API" + targetID,
			Message:    message,
			Connection: connection,
			Carnet:     carnet,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(out)
	}
}

func main() {
	apiID = os.Getenv("API_ID")   // 1, 2 o 3
	vmID = os.Getenv("VM_ID")     // VM1, VM2 o VM3
	port = os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	// Configura los peers según cuál API es esta
	switch apiID {
	case "1":
		peerURL["2"] = os.Getenv("API2_URL")
		peerVM["2"] = os.Getenv("API2_VM")
		peerURL["3"] = os.Getenv("API3_URL")
		peerVM["3"] = os.Getenv("API3_VM")
	case "2":
		peerURL["1"] = os.Getenv("API1_URL")
		peerVM["1"] = os.Getenv("API1_VM")
		peerURL["3"] = os.Getenv("API3_URL")
		peerVM["3"] = os.Getenv("API3_VM")
	case "3":
		peerURL["1"] = os.Getenv("API1_URL")
		peerVM["1"] = os.Getenv("API1_VM")
		peerURL["2"] = os.Getenv("API2_URL")
		peerVM["2"] = os.Getenv("API2_VM")
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", healthHandler)

	for targetID := range peerURL {
		path := fmt.Sprintf("GET /api%s/%s/call-api%s", apiID, carnet, targetID)
		mux.HandleFunc(path, makeCallHandler(targetID))
	}

	log.Printf("API%s escuchando en el puerto %s (VM: %s)", apiID, port, vmID)
	log.Fatal(http.ListenAndServe(":"+port, mux))
}
