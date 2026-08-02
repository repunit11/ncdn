package control

import (
	"fmt"
	"net/http"
	"net/netip"
	"time"

	"github.com/yzp0n/ncdn/l4lb/l4lbdrv"
)

type DataPlane interface {
	Apply(state l4lbdrv.ForwardingState) error
}

type Controller struct {
	dp           DataPlane
	desiredState l4lbdrv.ForwardingState
}

func New(dp DataPlane, desiredState l4lbdrv.ForwardingState) *Controller {
	return &Controller{dp, desiredState}
}

func (c *Controller) Reconcile() error {
	state := c.desiredState
	state.Dests = c.healthyDests()
	return c.dp.Apply(state)
}

func (c *Controller) healthyDests() l4lbdrv.DestinationEntries {
	if len(c.desiredState.Dests) == 0 {
		return nil
	}
	healthy := make(l4lbdrv.DestinationEntries, 0, len(c.desiredState.Dests))
	healthy = append(healthy, c.desiredState.Dests[0])
	for _, backend := range c.desiredState.Dests[1:] {
		if isHealthy(backend.IPAddr) {
			healthy = append(healthy, backend)
		}
	}
	return healthy
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
