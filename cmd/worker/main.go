// cmd/worker/main.go
package main

import (
	"log"
	"time"
)

func main() {
	log.Println("Worker started...")
	for {
		// placeholder job
		log.Println("Worker heartbeat")
		time.Sleep(30 * time.Second)
	}
}
