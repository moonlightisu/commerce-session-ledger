package main

import (
	"log"
	"net/http"
	"os"
)

func main() {
	key := os.Getenv("INFRAI_API_KEY")
	if key == "" {
		log.Fatal("INFRAI_API_KEY is required")
	}
	server := newCommerceServer(NewInfraiClient(key))
	log.Println("commerce service listening on :8080")
	log.Fatal(http.ListenAndServe(":8080", server.routes()))
}
