package main

import (
	"context"
	"flag"
	"log"
	"log/slog"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"github.com/yzp0n/ncdn/l4lb/dataplane"
	"github.com/yzp0n/ncdn/l4lb/l4lbdrv"
)

var lbBin = flag.String("lbBin", "c/lb.o", "Path to XDP lb binary")
var xdpcapHookPath = flag.String("xdpcapHookPath", "/sys/fs/bpf/xdpcap_hook", "Path to XDPCap hook")
var xdpif = flag.String("interface", "net0", "Interface to attach lb prog to")

func main() {
	flag.Parse()

	driverCfg := l4lbdrv.Config{
		BinPath:        *lbBin,
		XdpCapHookPath: *xdpcapHookPath,
		InterfaceName:  *xdpif,
	}

	dp, err := l4lbdrv.New(driverCfg)
	if err != nil {
		log.Panicf("Failed to create l4lb instance: %v", err)
	}
	slog.Info("L4LB started.")
	defer dp.Close()

	ctx, stop := signal.NotifyContext(
		context.Background(),
		syscall.SIGINT,
		syscall.SIGTERM,
	)
	defer stop()

	go dataplane.RunCounter(ctx, dp)

	server := &http.Server{
		Addr:    ":8080",
		Handler: dataplane.NewServer(dp),
	}
	serverErr := make(chan error, 1)
	go func() {
		serverErr <- server.ListenAndServe()
	}()

	select {
	case err := <-serverErr:
		if err != nil {
			log.Fatalf("HTTP server failed: %v", err)
		}
	case <-ctx.Done():
		shudownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := server.Shutdown(shudownCtx); err != nil {
			log.Printf("HTTP shutdown failed: %v", err)
		}
		slog.Info("Shutting down.")
	}
}
