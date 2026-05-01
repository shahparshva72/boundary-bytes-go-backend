package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/shahparshva72/boundary-bytes-go-backend/internal/app"
)

func main() {
	application, err := app.New()
	if err != nil {
		log.Fatalf("failed to initialize application: %v", err)
	}

	go func() {
		fmt.Printf("Server starting on :%s\n", application.Port())
		if err := application.Run(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("could not start server: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	fmt.Println("\nShutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if err := application.Shutdown(ctx); err != nil {
		log.Fatalf("forced shutdown: %v", err)
	}

	fmt.Println("Server exited gracefully")
}
