package app

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

// RunServer inicia el servidor HTTP y maneja el graceful shutdown ante
// SIGINT/SIGTERM. Es la única responsabilidad que queda en main.go.
func RunServer(srv *http.Server) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.ListenAndServe()
	}()

	select {
	case err := <-errCh:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			return err
		}
		return nil
	case <-ctx.Done():
		log.Println("Señal de apagado recibida, cerrando servidor...")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			log.Printf("Apagado forzado por timeout: %v", err)
			return err
		}
		log.Println("Servidor detenido")
		return nil
	}
}
