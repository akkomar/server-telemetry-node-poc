package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"test-go-publisher/glean"
	"time"
)

func main() {
	// Command-line flags
	projectID := flag.String("project", getEnv("PROJECT_ID", ""), "GCP project ID (required)")
	topicID := flag.String("topic", getEnv("TOPIC_ID", ""), "Pub/Sub topic name (required)")
	eventsPerSec := flag.Int("rate", getEnvInt("EVENTS_PER_SEC", 1), "Events per second to generate")
	duration := flag.Duration("duration", 0, "Run duration (0 = infinite)")
	flag.Parse()

	if *projectID == "" {
		log.Fatal("--project or PROJECT_ID environment variable is required")
	}

	if *topicID == "" {
		log.Fatal("--topic or TOPIC_ID environment variable is required")
	}

	log.Printf("Starting Glean Pub/Sub publisher")
	log.Printf("Configuration: project=%s, topic=%s, rate=%d events/sec, duration=%v",
		*projectID, *topicID, *eventsPerSec, *duration)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create publisher
	publisher, err := glean.NewGleanEventsPublisher(
		ctx,
		*projectID,
		*topicID,
		"accounts_backend",
		"0.0.1",
		"nightly",
	)
	if err != nil {
		log.Fatalf("Failed to create Pub/Sub publisher: %v", err)
	}
	defer publisher.Close()

	// Start stats reporting
	go func() {
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				stats := publisher.Stats()
				log.Printf("Stats: Published=%d, Errors=%d", stats.Published, stats.Errors)
			case <-ctx.Done():
				return
			}
		}
	}()

	// Graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigChan
		log.Println("Received shutdown signal, flushing pending messages...")
		publisher.Flush()
		os.Exit(0)
	}()

	// Event generation loop
	ticker := time.NewTicker(time.Second / time.Duration(*eventsPerSec))
	defer ticker.Stop()

	startTime := time.Now()
	eventCount := 0

	for {
		select {
		case <-ticker.C:
			requestInfo := glean.RequestInfo{}
			params := glean.EventsPing{
				IdentifiersFxaAccountId: fmt.Sprintf("account_%d", eventCount),
				Event: glean.BackendObjectUpdateEvent{
					ObjectType:  "api_request",
					ObjectState: fmt.Sprintf(`{"request_id": %d, "timestamp": "%s"}`, eventCount, time.Now().Format(time.RFC3339)),
					Linking:     eventCount%2 == 0,
				},
			}

			if err := publisher.RecordEventsPing(requestInfo, params); err != nil {
				log.Printf("Error recording event: %v", err)
			}

			eventCount++

			if *duration > 0 && time.Since(startTime) >= *duration {
				log.Printf("Duration limit reached, generated %d events", eventCount)
				publisher.Flush()
				return
			}

		case <-ctx.Done():
			log.Printf("Context cancelled, generated %d events", eventCount)
			return
		}
	}
}

// getEnv returns environment variable value or default
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// getEnvInt returns environment variable as int or default
func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		var intValue int
		if _, err := fmt.Sscanf(value, "%d", &intValue); err == nil {
			return intValue
		}
	}
	return defaultValue
}
