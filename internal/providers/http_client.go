package providers

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"sync"
	"syscall"
	"time"
)

type HTTPSettings struct {
	Timeout          time.Duration
	MaxRetries       int
	RetryBaseDelay   time.Duration
	CircuitThreshold int
	CircuitCooldown  time.Duration
}

func DefaultHTTPSettings() HTTPSettings {
	return HTTPSettings{
		Timeout:          20 * time.Second,
		MaxRetries:       2,
		RetryBaseDelay:   500 * time.Millisecond,
		CircuitThreshold: 3,
		CircuitCooldown:  30 * time.Second,
	}
}

type CircuitBreaker struct {
	mu              sync.Mutex
	consecutiveFail int
	openUntil       time.Time
	threshold       int
	cooldown        time.Duration
}

func NewCircuitBreaker(threshold int, cooldown time.Duration) *CircuitBreaker {
	if threshold <= 0 {
		threshold = 3
	}
	if cooldown <= 0 {
		cooldown = 30 * time.Second
	}

	return &CircuitBreaker{
		threshold: threshold,
		cooldown:  cooldown,
	}
}

func (c *CircuitBreaker) Allow() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	if now.Before(c.openUntil) {
		return fmt.Errorf("circuit breaker open until %s", c.openUntil.Format(time.RFC3339))
	}

	return nil
}

func (c *CircuitBreaker) RecordSuccess() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.consecutiveFail = 0
	c.openUntil = time.Time{}
}

func (c *CircuitBreaker) RecordFailure() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.consecutiveFail++
	if c.consecutiveFail >= c.threshold {
		c.openUntil = time.Now().Add(c.cooldown)
		c.consecutiveFail = 0
	}
}

func shouldRetryStatus(code int) bool {
	return code == http.StatusTooManyRequests || code >= 500
}

func shouldRetryError(err error) bool {
	if err == nil {
		return false
	}

	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, io.EOF) {
		return true
	}

	var netErr net.Error
	if errors.As(err, &netErr) && (netErr.Timeout() || netErr.Temporary()) {
		return true
	}

	if errors.Is(err, syscall.ECONNRESET) || errors.Is(err, syscall.ECONNREFUSED) {
		return true
	}

	return false
}

func sleepWithContext(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
