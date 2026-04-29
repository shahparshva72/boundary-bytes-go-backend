package main

import (
	"fmt"
	"log"

	"github.com/shahparshva72/boundary-bytes-go-backend/internal/app"
)

func main() {
	application, err := app.New()
	if err != nil {
		log.Fatalf("failed to initialize application: %v", err)
	}
	defer application.Close()

	fmt.Printf("Server starting on :%s\n", application.Port())

	if err := application.Run(); err != nil {
		log.Fatalf("could not start server: %v", err)
	}
}
