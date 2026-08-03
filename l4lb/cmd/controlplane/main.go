package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"log/slog"
	"net"
	"net/netip"
	"os/signal"
	"strings"
	"syscall"

	"github.com/yzp0n/ncdn/l4lb/control"
	"github.com/yzp0n/ncdn/l4lb/l4lbdrv"
)

var vip = flag.String("vip", "192.0.2.10", "VIP address to load balance")
var deststr = flag.String("dests", "", "Comma separated list of destination IP and MAC addresses. (Example: 192.168.88.10;00:00:5e:00:53:01,)")
var dataplaneURL = flag.String("dataplaneURL", "http://192.168.88.20:8080", "Dataplane URL")

func parseDest(deststr string) ([]l4lbdrv.DestinationEntry, error) {
	commas := strings.Split(deststr, ",")
	dests := make([]l4lbdrv.DestinationEntry, 0, len(commas))
	for _, c := range commas {
		if c == "" {
			continue
		}

		parts := strings.Split(c, ";")
		if len(parts) != 2 {
			return nil, fmt.Errorf("Invalid destination entry: %s", c)
		}
		ip4 := netip.MustParseAddr(parts[0])
		if ip4.Is6() {
			return nil, fmt.Errorf("Destination must be ipv4 address, but was %s", ip4)
		}

		mac, err := net.ParseMAC(parts[1])
		if err != nil {
			return nil, fmt.Errorf("Invalid MAC address: %s", parts[1])
		}

		dests = append(dests, l4lbdrv.DestinationEntry{
			IPAddr:       ip4,
			HardwareAddr: mac,
		})
	}
	log.Printf("dests: %+v", dests)
	return dests, nil
}

func main() {
	flag.Parse()

	dests, err := parseDest(*deststr)
	if err != nil {
		slog.Error("Failed to parse dest string", slog.String("err", err.Error()))
	}

	forwardingState := l4lbdrv.ForwardingState{
		VIP:   netip.MustParseAddr(*vip),
		Dests: dests,
	}

	dp := control.NewDataPlaneClient(*dataplaneURL)

	controller := control.New(dp, forwardingState)

	ctx, stop := signal.NotifyContext(
		context.Background(),
		syscall.SIGINT,
		syscall.SIGTERM,
	)
	defer stop()

	controller.Run(ctx)

	slog.Info("Shutting down.")
}
