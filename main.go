package main

import (
	"context"
	"github.com/joho/godotenv"
	"github.com/siegeai/siegelistener/integrations/siegeserver"
	"github.com/siegeai/siegelistener/listener"
	"log/slog"
	"os"
	"os/signal"
	"sync"
	"syscall"
)

// TODO We should be able to run / test the listener without access to the server
// TODO Wire loggers up in a sane way instead of this messy nonsense

func main() {
	_ = godotenv.Load()
	apikey := getEnv("SIEGE_APIKEY", "")
	device := getEnv("SIEGE_DEVICE", "lo")
	filter := getEnv("SIEGE_FILTER", "tcp and port 80")
	server := getEnv("SIEGE_SERVER", "https://dashboard.siegeai.com")
	level := getEnv("SIEGE_LOG", "info")

	err := setupLogging(level)
	if err != nil {
		slog.Error("could not init logging", "err", err)
		return
	}

	if apikey == "" {
		slog.Error("missing required config option SIEGE_APIKEY")
		return
	}

	source, err := listener.NewPacketSourceLive(device, filter)
	if err != nil {
		slog.Error("could not init packet source", "err", err)
		return
	}

	client, err := siegeserver.NewClient(apikey, server)
	if err != nil {
		slog.Error("could not init client", "err", err)
		return
	}

	l, err := listener.NewListener(source, client)
	if err != nil {
		slog.Error("could not init listener", "err", err)
		return
	}

	term := make(chan os.Signal, 1)
	signal.Notify(term, syscall.SIGINT, syscall.SIGTERM)

	err = l.RegisterStartup()
	if err != nil {
		return
	}
	defer l.RegisterShutdown()

	wg := &sync.WaitGroup{}
	defer wg.Wait()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	wg.Add(3)
	go l.ListenJob(ctx, wg)
	go l.PublishJob(ctx, wg)
	go l.ReassembleJob(ctx, wg)

	slog.Info("listening", "device", device, "filter", filter)
	<-term
}

func setupLogging(level string) error {
	var logLevel slog.Level
	err := logLevel.UnmarshalText([]byte(level))
	h := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: logLevel})
	slog.SetDefault(slog.New(h))
	return err
}

func getEnv(key, fallback string) string {
	if val, ok := os.LookupEnv(key); ok {
		return val
	}
	return fallback
}
