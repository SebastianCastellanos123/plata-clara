package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/SebastianCastellanos123/plata-clara/backend/internal/config"
	"github.com/SebastianCastellanos123/plata-clara/backend/internal/db"
	"github.com/SebastianCastellanos123/plata-clara/backend/internal/server"
)

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	if err := run(log); err != nil {
		log.Error("el servidor terminó con error", "err", err)
		os.Exit(1)
	}
}

func run(log *slog.Logger) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	// ctx se cancela cuando llega Ctrl+C o una señal de apagado de Docker.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	conn, err := db.Open(ctx, cfg.DatabaseURL, log)
	if err != nil {
		return err
	}
	defer conn.Close()

	if err := db.Migrate(ctx, conn, log); err != nil {
		return err
	}

	handler, err := server.New(cfg, conn, log)
	if err != nil {
		return err
	}

	// Timeouts: sin ellos, un cliente lento (ataque "slowloris") puede mantener
	// conexiones abiertas para siempre y agotar los recursos del servidor.
	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    1 << 16,
	}

	errCh := make(chan error, 1)
	go func() {
		log.Info("servidor escuchando", "addr", srv.Addr, "production", cfg.Production)
		errCh <- srv.ListenAndServe()
	}()

	select {
	case err := <-errCh:
		if !errors.Is(err, http.ErrServerClosed) {
			return err
		}
	case <-ctx.Done():
		log.Info("apagando...")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return srv.Shutdown(shutdownCtx) // termina las peticiones en curso antes de cerrar
	}
	return nil
}
