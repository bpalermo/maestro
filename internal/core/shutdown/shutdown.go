package shutdown

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"k8s.io/klog/v2"
)

type Shutdown interface {
	Shutdown(shutdownCtx context.Context) error
}

func AddShutdownHook(ctx context.Context, l klog.Logger, timeout time.Duration, shutdowns ...Shutdown) {
	l.Info("listening signals...")
	signal.NotifyContext(ctx,
		os.Interrupt, syscall.SIGHUP, syscall.SIGINT, syscall.SIGQUIT, syscall.SIGTERM,
	)

	<-ctx.Done()
	l.Info("Graceful shutdown...")

	shutdownCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	for _, shutdown := range shutdowns {
		if err := shutdown.Shutdown(shutdownCtx); err != nil {
			l.Error(err, "failed to stop closer")
		}
	}
	l.Info("Completed graceful shutdown")
	klog.FlushAndExit(klog.ExitFlushTimeout, 0)
}
