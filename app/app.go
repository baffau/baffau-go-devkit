package app

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"
)

var (
	// DefaultGracePeriod is the default value for the grace period.
	// During normal shutdown procedures, the shutdown function will wait
	// this amount of time before actually starting calling the shutdown handlers.
	DefaultGracePeriod = 3 * time.Second
	// DefaultShutdownTimeout is the default value for the timeout during shutdown.
	DefaultShutdownTimeout = 5 * time.Second
	// This is the default app.
	defaultApp *App
)

type ShutdownHandler func(context.Context) error

type MainLoopFunc func() error

// App represents an application with a main loop and a shutdown routine
type App struct {
	GracePeriod      time.Duration
	ShutdownTimeout  time.Duration
	shutdownHandlers []ShutdownHandler
	logger           *slog.Logger
}

// NewDefaultApp creates and sets the default app.
func NewDefaultApp(ctx context.Context) {
	defaultApp = &App{
		logger: slog.New(
			slog.NewJSONHandler(os.Stdout, nil),
		),
	}
	defaultApp.GracePeriod = DefaultGracePeriod
	defaultApp.ShutdownTimeout = DefaultShutdownTimeout
}

func (a *App) RunAndWait(mainLoop MainLoopFunc) {
	if defaultApp == nil {
		panic("default app not initialized")
	}
	a.logger.Info("[app] Starting run and wait.")

	errs := make(chan error)

	go func() {
		defer func() {
			recover()
		}()

		a.logger.Info("Application main loop starting now!")
		if mainLoop == nil {
			errs <- errors.New("main loop is nil")
			return
		}
		errs <- mainLoop()
	}()

	notifyCtx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	var err error
	ctx := context.Background()
	select {
	case <-notifyCtx.Done():
		a.logger.Info("Graceful shutdown signal received! Awaiting for grace period to end.")
		time.Sleep(a.GracePeriod)
		a.logger.Info("Grace period is over, initiating shutdown procedures...")
		err = a.Shutdown(ctx)
	case err = <-errs:
		a.logger.Error("Main Loop finished by itself, initiating shutdown procedures...",
			slog.String("error", err.Error()))
		err = a.Shutdown(ctx)
	}
	if err == nil {
		a.logger.Info("App gracefully terminated.")
	} else {
		a.logger.Error("App terminated with error",
			slog.String("error", err.Error()))
	}
}

// Shutdown calls all shutdown methods, in order they were added.
func (a *App) Shutdown(ctx context.Context) error {
	if defaultApp == nil {
		panic("default app not initialized")
	}

	for _, shutdownHandler := range a.shutdownHandlers {
		err := shutdownHandler(ctx)
		if err != nil {
			a.logger.Error("error executing shutdown handler",
				slog.String("module", "app/app"),
				slog.String("source", "app.Shutdown"),
				slog.String("error", err.Error()),
			)
		}
	}

	return nil
}

// RegisterShutdownHandler calls the RegisterShutdownHandler from the default app
func (a *App) RegisterShutdownHandler(handler ShutdownHandler) {
	if defaultApp == nil {
		panic("default app not initialized")
	}

	a.shutdownHandlers = append(a.shutdownHandlers, handler)
}
