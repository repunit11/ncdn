package control

import (
	"net"
	"net/netip"

	"github.com/yzp0n/ncdn/l4lb/l4lbdrv"
)

type DataPlane interface {
	Apply(state l4lbdrv.ForwardingState) error
}

type DestinationEntries []DestinationEntry

type DestinationEntry struct {
	IPAddr       netip.Addr
	HardwareAddr net.HardwareAddr
}

type Controller struct {
	dp           DataPlane
	desiredState l4lbdrv.ForwardingState
}

func New(dp DataPlane, desiredState l4lbdrv.ForwardingState) *Controller {
	return &Controller{dp, desiredState}
}

func (c *Controller) Reconcile() error {
	return c.dp.Apply(c.desiredState)
}
