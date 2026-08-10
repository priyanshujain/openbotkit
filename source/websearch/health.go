package websearch

import (
	"log/slog"
	"sync"
	"time"
)

const (
	transientMinCooldown = 5 * time.Second
	transientMaxCooldown = 60 * time.Second

	rateLimitMinCooldown = 60 * time.Second
	rateLimitMaxCooldown = 5 * time.Minute

	accessDeniedCooldown = 5 * time.Minute
)

type engineState int

const (
	stateHealthy engineState = iota
	stateUnhealthy
	stateHalfOpen // cooldown expired, allow one probe
)

type failureInfo struct {
	count    int
	lastFail time.Time
	cooldown time.Duration
	kind     FailureKind
	state    engineState
}

type healthTracker struct {
	mu       sync.Mutex
	backends map[string]*failureInfo
}

func newHealthTracker() *healthTracker {
	return &healthTracker{backends: make(map[string]*failureInfo)}
}

func (h *healthTracker) IsHealthy(name string) bool {
	h.mu.Lock()
	defer h.mu.Unlock()

	info, ok := h.backends[name]
	if !ok {
		return true
	}

	switch info.state {
	case stateHealthy, stateHalfOpen:
		return true // stateHealthy is the zero value; kept for safety
	case stateUnhealthy:
		if time.Since(info.lastFail) > info.cooldown {
			info.state = stateHalfOpen
			slog.Info("engine entering half-open state", "engine", name)
			return true
		}
		return false
	}
	return false
}

func (h *healthTracker) RecordFailure(name string, kind FailureKind) {
	h.mu.Lock()
	defer h.mu.Unlock()

	info, ok := h.backends[name]
	if !ok {
		info = &failureInfo{cooldown: minCooldownFor(kind)}
		h.backends[name] = info
	}

	wasHalfOpen := info.state == stateHalfOpen

	info.count++
	info.lastFail = time.Now()
	info.kind = kind
	info.state = stateUnhealthy

	if wasHalfOpen {
		info.cooldown *= 2
	} else if info.count > 1 {
		info.cooldown *= 2
	}

	min := minCooldownFor(kind)
	max := maxCooldownFor(kind)
	if info.cooldown < min {
		info.cooldown = min
	}
	if info.cooldown > max {
		info.cooldown = max
	}

	slog.Info("engine marked unhealthy", "engine", name, "kind", kind, "cooldown", info.cooldown)
}

func (h *healthTracker) RecordSuccess(name string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if _, ok := h.backends[name]; ok {
		slog.Info("engine recovered", "engine", name)
	}
	delete(h.backends, name)
}

func minCooldownFor(kind FailureKind) time.Duration {
	switch kind {
	case FailureRateLimit:
		return rateLimitMinCooldown
	case FailureAccessDenied:
		return accessDeniedCooldown
	default:
		return transientMinCooldown
	}
}

func maxCooldownFor(kind FailureKind) time.Duration {
	switch kind {
	case FailureRateLimit:
		return rateLimitMaxCooldown
	case FailureAccessDenied:
		return accessDeniedCooldown
	default:
		return transientMaxCooldown
	}
}
