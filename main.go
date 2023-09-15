package main

import (
	"context"
	"flag"
	"github.com/siegeai/siegelistener/listener"
	"log/slog"
	"os"
	"os/signal"
	"sync"
	"syscall"
)

func main() {
	dev := flag.String("i", "eth0", "Device to get packets from")
	port := flag.Int("p", 80, "Port to listen on")
	level := flag.String("l", "info", "Log level")
	flag.Parse()

	var logLevel slog.Level
	err := logLevel.UnmarshalText([]byte(*level))
	h := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: logLevel})
	slog.SetDefault(slog.New(h))

	slog.Info("init", "device", *dev, "port", *port, "logLevel", logLevel)
	if err != nil {
		slog.Warn("unrecognized log level", "l", *level)
	}

	s, err := listener.NewPacketSourceLive(*dev, *port)
	if err != nil {
		slog.Error("Could not create packet source", "err", err)
		return
	}

	term := make(chan os.Signal, 1)
	signal.Notify(term, syscall.SIGINT, syscall.SIGTERM)

	wg := &sync.WaitGroup{}
	defer wg.Wait()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	l := listener.NewListener(s)
	wg.Add(1)
	go l.Listen(ctx, wg)

	<-term
}
