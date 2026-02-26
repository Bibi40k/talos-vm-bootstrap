package ssh

import (
	"context"
	"fmt"
	"net"
	"time"
)

type TCPCheckStats struct {
	Attempts int
	Elapsed  time.Duration
}

func WaitForTCPPort(ctx context.Context, host string, port int, attempts int, connectTimeout time.Duration, retryDelay time.Duration) error {
	_, err := WaitForTCPPortWithStats(ctx, host, port, attempts, connectTimeout, retryDelay)
	return err
}

func WaitForTCPPortWithStats(ctx context.Context, host string, port int, attempts int, connectTimeout time.Duration, retryDelay time.Duration) (TCPCheckStats, error) {
	started := time.Now()
	stats := TCPCheckStats{}
	address := net.JoinHostPort(host, fmt.Sprintf("%d", port))
	var lastErr error

	for i := 0; i < attempts; i++ {
		stats.Attempts = i + 1
		d := net.Dialer{Timeout: connectTimeout}
		conn, err := d.DialContext(ctx, "tcp", address)
		if err == nil {
			_ = conn.Close()
			stats.Elapsed = time.Since(started)
			return stats, nil
		}
		lastErr = err

		select {
		case <-ctx.Done():
			stats.Elapsed = time.Since(started)
			return stats, fmt.Errorf("ssh connectivity canceled after %d attempts in %s: %w", stats.Attempts, stats.Elapsed.Truncate(time.Millisecond), ctx.Err())
		case <-time.After(retryDelay):
		}
	}

	stats.Elapsed = time.Since(started)
	return stats, fmt.Errorf("ssh connectivity failed after %d attempts in %s: %w", attempts, stats.Elapsed.Truncate(time.Millisecond), lastErr)
}
