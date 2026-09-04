package main

import (
	"context"
	"fmt"
	"time"
)

// Run safely calls f, retrying up to maxAttempts times within maxDuration.
// The context passed to f is cancelled when the deadline expires, allowing f
// to stop gracefully instead of leaking.
func Run(label string, maxDuration time.Duration, maxAttempts int, f func(ctx context.Context) error) bool {
	fmt.Printf("########## %s ##########\n", label)

	if maxAttempts > 30 {
		maxAttempts = 30
	}
	ctx, cancel := context.WithTimeout(context.Background(), maxDuration)
	defer cancel()

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		errCh := make(chan error, 1)
		go func() {
			defer func() {
				if r := recover(); r != nil {
					errCh <- fmt.Errorf("%s panicked: %v", label, r)
				}
			}()
			errCh <- f(ctx)
		}()

		select {
		case err := <-errCh:
			if err == nil {
				return true
			}
			fmt.Printf("%s failed: %s\n", label, err)
		case <-ctx.Done():
			fmt.Printf("%s failed after %d attempts (timed out)\n", label, attempt)
			return false
		}

		backoff := time.Duration(1<<attempt) * 10 * time.Millisecond
		select {
		case <-time.After(backoff):
		case <-ctx.Done():
			fmt.Printf("%s failed after %d attempts (timed out)\n", label, attempt)
			return false
		}
	}

	fmt.Printf("%s failed after %d attempts\n", label, maxAttempts)
	return false
}
