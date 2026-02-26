package ssh

import (
	"context"
	"net"
	"testing"
	"time"
)

func TestWaitForTCPPortWithStatsSuccess(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer func() { _ = ln.Close() }()

	host, portStr, err := net.SplitHostPort(ln.Addr().String())
	if err != nil {
		t.Fatalf("split host port: %v", err)
	}
	port, err := net.LookupPort("tcp", portStr)
	if err != nil {
		t.Fatalf("lookup port: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	stats, err := WaitForTCPPortWithStats(ctx, host, port, 2, 500*time.Millisecond, 50*time.Millisecond)
	if err != nil {
		t.Fatalf("expected success, got %v", err)
	}
	if stats.Attempts < 1 {
		t.Fatalf("expected attempts >= 1")
	}
}

func TestWaitForTCPPortWithStatsFailure(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 400*time.Millisecond)
	defer cancel()

	_, err := WaitForTCPPortWithStats(ctx, "127.0.0.1", 1, 2, 50*time.Millisecond, 50*time.Millisecond)
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestWaitForTCPPort(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer func() { _ = ln.Close() }()

	host, portStr, err := net.SplitHostPort(ln.Addr().String())
	if err != nil {
		t.Fatalf("split host port: %v", err)
	}
	port, err := net.LookupPort("tcp", portStr)
	if err != nil {
		t.Fatalf("lookup port: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := WaitForTCPPort(ctx, host, port, 2, 500*time.Millisecond, 50*time.Millisecond); err != nil {
		t.Fatalf("expected success, got %v", err)
	}
}
