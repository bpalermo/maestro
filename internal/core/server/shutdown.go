package server

import (
	"context"
	"io"
	"os"
	"os/signal"
	"syscall"

	"github.com/rs/zerolog/log"
)

func AddShutdownHook(ctx context.Context, closers ...io.Closer) {
	log.Info().Msg("listening signals...")
	shutdownCtx, stopCtx := signal.NotifyContext(
		ctx,
		os.Interrupt,
		syscall.SIGHUP,
		syscall.SIGINT,
		syscall.SIGQUIT,
		syscall.SIGTERM,
	)
	defer stopCtx()

	<-shutdownCtx.Done()
	log.Info().Msg("graceful shutdown...")

	for _, closer := range closers {
		if err := closer.Close(); err != nil {
			log.Err(err).Msg("failed to stop closer")
		}
	}
	ctx.Done()

	log.Info().Msg("completed graceful shutdown")
}
