package postgres

import (
	"context"
	"log"
	"os"
	"testing"
	"time"
)

func TestMain(m *testing.M) {
	log.Println("Starting Postgres module tests...")
	// Package timeout context
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute*5)
	defer cancel()

	_, teardownContainer := createTestContainer(ctx)
	defer teardownContainer()

	// Run tests in a goroutine so we can monitor for timeout
	done := make(chan int, 1)
	go func() {
		done <- m.Run()
	}()

	var code int
	select {
	case code = <-done:
		// Tests completed normally
	case <-ctx.Done():
		// Timeout occurred
		code = 1
		println("Test execution timed out after 5 minutes")
	}

	os.Exit(code)
}
