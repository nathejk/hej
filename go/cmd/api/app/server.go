package app

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

// Serve runs an HTTP server on the given port with sane timeouts and graceful
// shutdown on SIGINT/SIGTERM.
//
// # The timeouts are sized for a phone in a forest, not for a datacentre
//
// `ReadTimeout` covers reading the **entire request, including the body**. It used to be
// 5 seconds, which is fine for JSON and quietly wrong for an upload: a portrait over a
// weak mobile link needs far longer than that, so the server would abort the read and the
// member would see an upload that "just fails sometimes" — intermittently, in the field,
// which is the worst possible place to debug it.
//
// Rather than loosen it for everything, the values below stay modest and the one endpoint
// that needs minutes extends its own deadline (see updatePhotoHandler). What is loosened
// generally is `WriteTimeout`, because in production this binary also serves the SPA
// bundle, and 10 seconds assumed roughly 60 KB/s.
//
// `ReadHeaderTimeout` is set explicitly so that relaxing `ReadTimeout` does not relax the
// slowloris protection with it: headers still have to arrive promptly.
func (a *JsonApi) Serve(handler http.Handler, port int) error {
	srv := &http.Server{
		Addr:              fmt.Sprintf(":%d", port),
		Handler:           handler,
		IdleTimeout:       time.Minute,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      60 * time.Second,
	}

	shutdownError := make(chan error)

	go func() {
		quit := make(chan os.Signal, 1)
		signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
		s := <-quit

		a.Logger.Info("shutting down server", "signal", s.String())

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		shutdownError <- srv.Shutdown(ctx)
	}()

	a.Logger.Info("starting server", "addr", srv.Addr)

	err := srv.ListenAndServe()
	if !errors.Is(err, http.ErrServerClosed) {
		return err
	}

	if err := <-shutdownError; err != nil {
		return err
	}

	a.Logger.Info("stopped server", "addr", srv.Addr)
	return nil
}
