package main

import (
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/leofds/conduit/internal/config"
	"github.com/leofds/conduit/internal/resolver"
	"github.com/leofds/conduit/internal/resolver/apiresolver"
	"github.com/leofds/conduit/internal/resolver/fileresolver"
	"github.com/leofds/conduit/internal/server"
	"github.com/leofds/conduit/internal/version"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	var r resolver.Resolver
	switch cfg.Resolver {
	case config.ResolverAPI:
		r, err = apiresolver.New(apiresolver.Config{
			URL:             cfg.API.URL,
			ConnectTimeout:  cfg.API.ConnectTimeout,
			ResponseTimeout: cfg.API.ResponseTimeout,
		})
		if err != nil {
			log.Fatalf("API resolver: %v", err)
		}
		log.Printf("Resolver: api → %s", cfg.API.URL)
	default: // ResolverFile
		fr, err := fileresolver.New(cfg.Local)
		if err != nil {
			log.Fatalf("Failed to load host config: %v", err)
		}
		r = fr
		log.Printf("Resolver: file")
	}

	srv := server.New(r)
	srv.SetAllowLocal(cfg.Local.Enable)
	srv.SetDemo(cfg.Demo)
	srv.SetTerm(cfg.Local.Term)

	go func() {
		log.Printf("Starting conduit %s on :8080", version.Version)
		if err := srv.Start(":8080"); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("Server error: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down...")
	if err := srv.Shutdown(); err != nil {
		log.Fatalf("Shutdown error: %v", err)
	}
	log.Println("Stopped")
}
