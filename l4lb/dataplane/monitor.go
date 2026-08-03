package dataplane

import (
	"context"
	"log/slog"
	"time"
)

type CounterDumper interface {
	DumpCounters() error
}

func RunCounter(ctx context.Context, dumper CounterDumper) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if err := dumper.DumpCounters(); err != nil {
				slog.Error("Failed to dump counters", "err", err)
			}
		case <-ctx.Done():
			return
		}
	}
}
