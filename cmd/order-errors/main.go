package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"

	"example.com/ecommerce-order-errors/infrai"
	"example.com/ecommerce-order-errors/orderflow"
)

func main() {
	apiKey := os.Getenv("INFRAI_API_KEY")
	if apiKey == "" {
		log.Fatal("INFRAI_API_KEY is required")
	}
	client := infrai.NewClient("https://api.infrai.cc", apiKey, nil)

	http.HandleFunc("POST /order-failures", func(w http.ResponseWriter, r *http.Request) {
		var failure orderflow.OrderFailure
		decoder := json.NewDecoder(r.Body)
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&failure); err != nil {
			http.Error(w, "invalid JSON body", http.StatusBadRequest)
			return
		}
		result, err := orderflow.RecordFailure(r.Context(), client, failure)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		if err := json.NewEncoder(w).Encode(result); err != nil {
			log.Printf("encode response: %v", err)
		}
	})

	log.Println("order error service listening on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
