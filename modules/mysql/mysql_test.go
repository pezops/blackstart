package mysql

import (
	"context"
	"log"
	"os"
	"testing"
	"time"
)

// TestMain starts the shared MySQL test container before running package tests.
func TestMain(m *testing.M) {
	log.Println("Starting MySQL module tests...")
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute*5)
	defer cancel()

	_, teardownContainer := createTestContainer(ctx)
	defer teardownContainer()

	done := make(chan int, 1)
	go func() {
		done <- m.Run()
	}()

	var code int
	select {
	case code = <-done:
	case <-ctx.Done():
		log.Fatalf("Test execution timed out after 5 minutes")
	}

	os.Exit(code)
}
