package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/neoncorp/Whatskel/bot"
	"github.com/neoncorp/Whatskel/config"
)

func main() {
	cfgPath := flag.String("config", "config.toml", "path to configuration file")
	flag.Parse()

	cfg, err := config.Load(*cfgPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	b, err := bot.New(cfg)
	if err != nil {
		log.Fatalf("Failed to initialize bot: %v", err)
	}

	if err := b.Start(); err != nil {
		log.Fatalf("Failed to start bot: %v", err)
	}

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	<-sig

	log.Println("Shutting down...")
	b.Stop()
}
