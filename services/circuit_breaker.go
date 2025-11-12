package services

import (
	"fmt"
	"log"
	"sync"
	"time"
)

// CircuitBreaker implements the circuit breaker pattern to prevent cascading failures
type CircuitBreaker struct {
	name        string
	maxFailures int
	cooldown    time.Duration
	failures    int
	lastFailure time.Time
	isOpen      bool
	mu          sync.RWMutex
}

// NewCircuitBreaker creates a new circuit breaker
func NewCircuitBreaker(name string, maxFailures int, cooldown time.Duration) *CircuitBreaker {
	return &CircuitBreaker{
		name:        name,
		maxFailures: maxFailures,
		cooldown:    cooldown,
		failures:    0,
		isOpen:      false,
	}
}

// Call executes the given function with circuit breaker protection
func (cb *CircuitBreaker) Call(fn func() error) error {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	// Check if circuit is open
	if cb.isOpen {
		if time.Since(cb.lastFailure) > cb.cooldown {
			// Try half-open state
			cb.isOpen = false
			cb.failures = 0
			log.Printf("[CircuitBreaker:%s] Attempting half-open state", cb.name)
		} else {
			return fmt.Errorf("circuit breaker %s is open (cooldown until %v)",
				cb.name, cb.lastFailure.Add(cb.cooldown))
		}
	}

	err := fn()

	if err != nil {
		cb.failures++
		cb.lastFailure = time.Now()

		if cb.failures >= cb.maxFailures {
			cb.isOpen = true
			log.Printf("🔴 [CircuitBreaker:%s] OPENED after %d failures (cooldown: %v)",
				cb.name, cb.failures, cb.cooldown)
		}

		return err
	}

	// Success - reset counter
	if cb.failures > 0 {
		log.Printf("✅ [CircuitBreaker:%s] Closed (recovered after %d failures)", cb.name, cb.failures)
	}
	cb.failures = 0
	return nil
}

// IsOpen returns true if the circuit breaker is currently open
func (cb *CircuitBreaker) IsOpen() bool {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.isOpen
}

// Reset manually resets the circuit breaker
func (cb *CircuitBreaker) Reset() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.failures = 0
	cb.isOpen = false
	log.Printf("[CircuitBreaker:%s] Manually reset", cb.name)
}
