package control

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"net/netip"
	"sync"
	"time"

	"github.com/yzp0n/ncdn/l4lb/l4lbdrv"
)

type DataPlane interface {
	Apply(state l4lbdrv.ForwardingState) error
}

const (
	healthyThreshold   = 2
	unhealthyThreshold = 3
)

type BackendHealth struct {
	Healthy             bool
	ConsecutiveSuccess  int
	ConsecutiveFailures int
}

type Controller struct {
	dp            DataPlane
	desiredState  l4lbdrv.ForwardingState
	backendHealth map[netip.Addr]*BackendHealth
}

func New(dp DataPlane, desiredState l4lbdrv.ForwardingState) *Controller {
	return &Controller{
		dp:            dp,
		desiredState:  desiredState,
		backendHealth: make(map[netip.Addr]*BackendHealth),
	}
}

func (c *Controller) Run(ctx context.Context) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	if err := c.reconcile(); err != nil {
		slog.Error("Failed to reconcile", "err", err)
	}

	for {
		select {
		case <-ticker.C:
			if err := c.reconcile(); err != nil {
				slog.Error("Failed to reconcile", "err", err)
			}
		case <-ctx.Done():
			return
		}
	}
}

func (c *Controller) reconcile() error {
	state := c.desiredState
	state.Dests = c.healthyDests()
	return c.dp.Apply(state)
}

func (c *Controller) healthyDests() l4lbdrv.DestinationEntries {
	if len(c.desiredState.Dests) == 0 {
		return nil
	}

	backends := c.desiredState.Dests[1:]
	checkResults := make([]bool, len(backends))

	type healthCheckResult struct {
		index     int
		succeeded bool
	}

	results := make(chan healthCheckResult, len(backends))
	var wg sync.WaitGroup

	for index, backend := range backends {
		wg.Add(1)
		go func(index int, ip netip.Addr) {
			defer wg.Done()
			results <- healthCheckResult{
				index:     index,
				succeeded: isHealthy(ip),
			}
		}(index, backend.IPAddr)
	}

	wg.Wait()
	close(results)

	for result := range results {
		checkResults[result.index] = result.succeeded
	}

	healthy := make(l4lbdrv.DestinationEntries, 0, len(c.desiredState.Dests))
	healthy = append(healthy, c.desiredState.Dests[0])
	for index, backend := range backends {
		health := c.updateBackendHealth(backend.IPAddr, checkResults[index])
		if health.Healthy {
			healthy = append(healthy, backend)
		}
	}
	return healthy
}

func (c *Controller) updateBackendHealth(ip netip.Addr, checkSucceeded bool) *BackendHealth {
	health, ok := c.backendHealth[ip]
	if !ok {
		health = &BackendHealth{}
		c.backendHealth[ip] = health
	}

	wasHealthy := health.Healthy

	if checkSucceeded {
		health.ConsecutiveFailures = 0
		if health.ConsecutiveSuccess < healthyThreshold {
			health.ConsecutiveSuccess++
		}
		if health.ConsecutiveSuccess >= healthyThreshold {
			health.Healthy = true
		}
	} else {
		health.ConsecutiveSuccess = 0
		if health.ConsecutiveFailures < unhealthyThreshold {
			health.ConsecutiveFailures++
		}
		if health.ConsecutiveFailures >= unhealthyThreshold {
			health.Healthy = false
		}
	}

	if wasHealthy != health.Healthy {
		slog.Info(
			"Backend health changed",
			"ip", ip,
			"healthy", health.Healthy,
			"consecutiveSuccess", health.ConsecutiveSuccess,
			"consecutiveFailures", health.ConsecutiveFailures,
		)
	}

	return health
}

func isHealthy(ip netip.Addr) bool {
	client := &http.Client{
		Timeout: time.Second,
	}
	url := fmt.Sprintf("http://%s:8889/statusz", ip)
	resp, err := client.Get(url)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	return resp.StatusCode == http.StatusOK
}
