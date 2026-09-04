package main

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestRunTrivialSuccess(t *testing.T) {
	f := func(ctx context.Context) error {
		return nil
	}
	ok := Run("test", time.Second, 1, f)
	if !ok {
		t.Fatal("trivial success run failed")
	}
}

func TestRetriesOnce(t *testing.T) {
	fCalls := 0
	f := func(ctx context.Context) error {
		defer func() { fCalls += 1 }()
		// Fails on first try
		if fCalls == 0 {
			return errors.New("BOOM")
		}
		// Succeeds on second try
		return nil
	}
	ok := Run("test", time.Second, 2, f)
	if !ok {
		t.Fatal("failed to retry")
	}
	if fCalls != 2 {
		t.Fatalf("unexpected num of f calls: %d", fCalls)
	}
}

func TestDoesntRetryForever(t *testing.T) {
	fCalls := 0
	f := func(ctx context.Context) error {
		// Always fails
		defer func() { fCalls += 1 }()
		return errors.New("BOOM")
	}
	// Despite a huge max time, will only retry for the max num of times specified
	ok := Run("test", time.Hour, 2, f)
	if ok {
		t.Fatal("returned ok even on forced failure")
	}
	if fCalls != 2 {
		t.Fatalf("unexpected num of f calls: %d", fCalls)
	}
}
