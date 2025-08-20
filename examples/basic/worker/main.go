package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	queueProvider := os.Getenv("QUEUE_PROVIDER")
	if queueProvider == "" {
		queueProvider = "memory"
	}

	log.Printf("Worker starting with queue provider: %s", queueProvider)

	// graceful shutdown
	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, os.Interrupt, syscall.SIGTERM)

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	taskCount := 0

	for {
		select {
		case <-ticker.C:
			taskCount++
			log.Printf("Processing task #%d using %s queue...", taskCount, queueProvider)
			// simulate some work
			time.Sleep(100 * time.Millisecond)
			log.Printf("Task #%d completed", taskCount)

		case <-shutdown:
			log.Println("Worker shutting down gracefully...")
			// simulate cleanup
			time.Sleep(500 * time.Millisecond)
			log.Println("Worker stopped")
			return
		}
	}
}
