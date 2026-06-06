package main

import (
	"embed"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/leofds/conduit/internal/config"
	"github.com/leofds/conduit/internal/knownhosts"
	"github.com/leofds/conduit/internal/resolver"
	"github.com/leofds/conduit/internal/resolver/apiresolver"
	"github.com/leofds/conduit/internal/resolver/fileresolver"
	"github.com/leofds/conduit/internal/server"
	"github.com/leofds/conduit/internal/version"
)

//go:embed defaults/conduit.yaml defaults/hosts.yaml
var defaultFiles embed.FS

// writeDefaultsIfMissing creates conduit.yaml and hosts.yaml from the embedded
// templates when none of the standard config locations contain the file.
// Must be called after the working directory has been set to the binary's
// directory (see chdirToBin). Existing files are never overwritten.
func writeDefaultsIfMissing() {
	checks := []struct {
		localName string
		paths     []string
	}{
		{"conduit.yaml", config.ConduitConfigPaths},
		{"hosts.yaml", config.HostsConfigPaths},
	}
	for _, c := range checks {
		found := false
		for _, path := range c.paths {
			if _, err := os.Stat(path); err == nil {
				found = true
				break
			}
		}
		if found {
			continue
		}
		data, err := defaultFiles.ReadFile("defaults/" + c.localName)
		if err != nil {
			log.Printf("warning: could not read embedded %s: %v", c.localName, err)
			continue
		}
		if err := os.WriteFile(c.localName, data, 0644); err != nil {
			log.Printf("warning: could not create %s: %v", c.localName, err)
			continue
		}
		log.Printf("created default %s", c.localName)
	}
}

// chdirToBin changes the working directory to the directory containing the
// binary so that all relative paths in config files resolve consistently,
// regardless of where conduit was invoked from.
func chdirToBin() {
	exe, err := os.Executable()
	if err != nil {
		log.Printf("warning: could not determine executable path: %v", err)
		return
	}
	exe, err = filepath.EvalSymlinks(exe)
	if err != nil {
		log.Printf("warning: could not resolve executable symlinks: %v", err)
		return
	}
	if err := os.Chdir(filepath.Dir(exe)); err != nil {
		log.Printf("warning: could not chdir to binary directory: %v", err)
	}
}

func main() {
	resetKnownHost := flag.String("R", "", "remove the stored SSH host key fingerprint for the given host and exit")
	flag.Parse()

	log.Printf("Conduit %s", version.Version)
	chdirToBin()
	writeDefaultsIfMissing()

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	if *resetKnownHost != "" {
		host := strings.TrimSpace(*resetKnownHost)
		ks, err := knownhosts.New(cfg.SSH.KnownHostsFile)
		if err != nil {
			log.Fatalf("Known hosts: %v", err)
		}
		if err := ks.Remove(host); err != nil {
			log.Fatalf("Failed to reset known host %q: %v", host, err)
		}
		log.Printf("Removed stored fingerprint for host %q", host)
		return
	}

	// Ensure no unexpected flags remain before starting the server.
	if flag.NArg() > 0 {
		log.Fatalf("unexpected arguments: %v", flag.Args())
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

	srv := server.New(r, cfg.Headers)
	srv.SetDebugBanner(cfg.DebugBanner)
	srv.SetAllowLocal(cfg.AllowLocalShell)
	srv.SetLocalConfig(cfg.Local)
	srv.SetDemo(cfg.Demo)
	srv.SetSSHConfig(cfg.SSH)
	srv.SetAllowedOrigins(cfg.AllowedOrigins)

	ks, err := knownhosts.New(cfg.SSH.KnownHostsFile)
	if err != nil {
		log.Fatalf("Known hosts: %v", err)
	}
	srv.SetKnownHosts(ks)

	addr := fmt.Sprintf(":%d", cfg.Port)
	go func() {
		log.Printf("Starting conduit %s on %s", version.Version, addr)
		if err := srv.Start(addr); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("Server error: %v", err)
		}
	}()

	// Handle SIGHUP for hosts file reload (only for file resolver)
	sighup := make(chan os.Signal, 1)
	signal.Notify(sighup, syscall.SIGHUP)
	go func() {
		for range sighup {
			if fr, ok := r.(*fileresolver.Resolver); ok {
				if err := fr.Reload(); err != nil {
					log.Printf("error reloading hosts: %v", err)
				} else {
					log.Printf("hosts reloaded successfully")
				}
			} else {
				log.Printf("SIGHUP received: ignoring (not using file resolver)")
			}
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
