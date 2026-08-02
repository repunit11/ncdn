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
	dp DataPlane
}

func New(dp DataPlane) *Controller {
	return &Controller{dp: dp}
}

func (c *Controller) Apply(state l4lbdrv.ForwardingState) error {
	return c.dp.Apply(state)
}
