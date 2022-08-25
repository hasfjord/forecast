package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/hasfjord/forecast/internal/forecast"
	"github.com/hasfjord/forecast/internal/influx"
	"github.com/hasfjord/forecast/internal/yr"
	"github.com/kelseyhightower/envconfig"
	"github.com/sirupsen/logrus"
)

type Config struct {
	YR       yr.Config
	TSDB     influx.Config
	Forecast forecast.Config
	Address  string `envconfig:"HTTP_ADDRESS" required:"true"`
	LogLevel string `envconfig:"LOG_LEVEL" default:"info"`
}

func main() {
	ctx, done := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	err := realMain(ctx)
	done()
	if err != nil {
		logrus.Fatal(err)
	}
}

func realMain(ctx context.Context) error {
	logrus.SetOutput(os.Stdout)

	logrus.Info("Starting server")

	var cfg Config
	if err := envconfig.Process("", &cfg); err != nil {
		_ = envconfig.Usage("", &cfg)
		logrus.WithError(err).Fatal("main: failed to load environment variables")
	}
	logrus.WithField("config", &cfg).Debug("main: loaded config")
	level, err := logrus.ParseLevel(cfg.LogLevel)
	if err != nil {
		return fmt.Errorf("failed to parse log level: %w", err)
	}
	logrus.SetLevel(level)

	ctx, cancel := context.WithCancel(ctx)

	var wg sync.WaitGroup

	dbClient := influx.NewClient(cfg.TSDB)

	yrClient := yr.NewClient(cfg.YR, &http.Client{})

	forecastServer := forecast.NewServer(yrClient, dbClient, cfg.Forecast)

	server := &http.Server{Addr: cfg.Address,
		ReadTimeout:       time.Second * 15,
		ReadHeaderTimeout: time.Second * 10,
		WriteTimeout:      time.Second * 10}

	http.HandleFunc("/forecast/run", forecastServer.MakeRunForecastHandler())
	http.HandleFunc("/readiness", HealthHandler)
	http.HandleFunc("/liveness", HealthHandler)

	go func() {
		logrus.Infof("Started listening on %s ...\n", cfg.Address)
		wg.Add(1)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logrus.WithError(err).Error("main: failed to gracefully shut down server")
		}
		cancel()
		wg.Done()
	}()

	// Wait for the main context to be done, meaning we have received a
	// shutdown signal
	<-ctx.Done()
	logrus.Info("worker: shutting down server...")

	// Shut down the server with a 5s timeout
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}

	// Wait for all goroutines to shut down
	logrus.Info("waiting for goroutines to finish...")
	wg.Wait()
	logrus.Info("Closed server successfully")

	return nil
}

// handler for health endpoints: /readiness and /liveness
func HealthHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
}
