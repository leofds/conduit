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

	conduit "github.com/leofds/conduit"
	"github.com/leofds/conduit/internal/config"
	iserver "github.com/leofds/conduit/internal/server"
	"github.com/leofds/conduit/internal/version"
	"github.com/leofds/conduit/knownhosts"
	"github.com/leofds/conduit/resolver"
	"github.com/leofds/conduit/resolver/apiresolver"
	"github.com/leofds/conduit/resolver/fileresolver"
	"github.com/leofds/conduit/session"
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

func parseFlags(args []string) (resetKnownHost string, writeDefaultFiles bool, remainingArgs []string, err error) {
	fs := flag.NewFlagSet("conduit", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	var reset string
	var writeDefaults bool
	fs.StringVar(&reset, "reset-known-host", "", "remove the stored SSH host key fingerprint for the given host and exit")
	fs.StringVar(&reset, "R", "", "short alias for -reset-known-host")
	fs.BoolVar(&writeDefaults, "write-defaults", false, "create conduit.yaml and hosts.yaml from embedded defaults if no config files are found in the standard locations")
	fs.BoolVar(&writeDefaults, "W", false, "short alias for -write-defaults")

	if err := fs.Parse(args); err != nil {
		return "", false, nil, err
	}

	return reset, writeDefaults, fs.Args(), nil
}

// buildResolver selects the resolver backend from the loaded configuration.
func buildResolver(cfg *config.Config) (resolver.Resolver, *fileresolver.Resolver, error) {
	switch cfg.Resolver {
	case config.ResolverAPI:
		r, err := apiresolver.New(apiresolver.Config{
			URL:             cfg.API.URL,
			ConnectTimeout:  cfg.API.ConnectTimeout,
			ResponseTimeout: cfg.API.ResponseTimeout,
		})
		if err != nil {
			return nil, nil, fmt.Errorf("API resolver: %w", err)
		}
		log.Printf("Resolver: api → %s", cfg.API.URL)
		return r, nil, nil
	default: // ResolverFile
		fr, err := fileresolver.New(cfg.Local)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to load host config: %w", err)
		}
		log.Printf("Resolver: file")
		return fr, fr, nil
	}
}

func main() {
	resetKnownHost, writeDefaultFiles, remainingArgs, err := parseFlags(os.Args[1:])
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return
		}
		log.Fatalf("Failed to parse flags: %v", err)
	}

	log.Printf("Conduit %s", version.Version)
	chdirToBin()
	if writeDefaultFiles {
		writeDefaultsIfMissing()
		return
	}

	cfg, err := config.Load(config.ConduitConfigPaths)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	if resetKnownHost != "" {
		host := strings.TrimSpace(resetKnownHost)
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
	if len(remainingArgs) > 0 {
		log.Fatalf("unexpected arguments: %v", remainingArgs)
	}

	r, fileResolver, err := buildResolver(cfg)
	if err != nil {
		log.Fatalf("%v", err)
	}

	ks, err := knownhosts.New(cfg.SSH.KnownHostsFile)
	if err != nil {
		log.Fatalf("Known hosts: %v", err)
	}

	sessMan := session.NewManager()

	srv, err := conduit.New(conduit.Options{
		Resolver:        r,
		KnownHosts:      ks,
		SessionManager:  sessMan,
		AllowLocal:      cfg.AllowLocalShell,
		Demo:            cfg.Demo,
		DebugBanner:     cfg.DebugBanner,
		AllowedOrigins:  cfg.AllowedOrigins,
		HTTPHeaders:     cfg.Headers,
		TerminalOptions: cfg.TerminalOptions,
		SSH: conduit.SSHOptions{
			Port:              cfg.SSH.Port,
			Term:              cfg.SSH.Term,
			IdleTimeout:       cfg.SSH.IdleTimeout,
			KeepaliveInterval: cfg.SSH.KeepaliveInterval,
			DialTimeout:       cfg.SSH.DialTimeout,
			VerifyHostKey:     cfg.SSH.VerifyHostKey,
			AutoAcceptHostKey: cfg.SSH.AutoAcceptHostKey,
			Env:               cfg.SSH.Env,
		},
		Local: conduit.LocalOptions{
			Command:     cfg.Local.Command,
			Term:        cfg.Local.Term,
			WorkingDir:  cfg.Local.WorkingDir,
			IdleTimeout: cfg.Local.IdleTimeout,
			Env:         cfg.Local.Env,
		},
		Timeouts: conduit.Timeouts{
			Read:        cfg.Server.Timeouts.Read,
			Write:       cfg.Server.Timeouts.Write,
			ReadHeader:  cfg.Server.Timeouts.ReadHeader,
			Idle:        cfg.Server.Timeouts.Idle,
			WSHandshake: cfg.Server.Timeouts.WSHandshake,
		},
	})
	if err != nil {
		log.Fatalf("Failed to create server: %v", err)
	}

	addr := fmt.Sprintf(":%d", cfg.Port)
	go func() {
		log.Printf("Starting conduit on %s", addr)
		if err := srv.Start(addr); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("Server error: %v", err)
		}
	}()

	control, err := iserver.NewControlServer(cfg.ControlSocket, sessMan)
	if err != nil {
		log.Fatalf("Failed to create control server: %v", err)
	}
	err = control.StartUnixSocket()
	if err != nil {
		log.Fatalf("Failed to start control server: %v", err)
	}

	// Handle SIGHUP for hosts file reload (only for file resolver)
	sighup := make(chan os.Signal, 1)
	signal.Notify(sighup, syscall.SIGHUP)
	go func() {
		for range sighup {
			if fileResolver != nil {
				if err := fileResolver.Reload(); err != nil {
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
	errSrv := srv.Shutdown()
	errControlSrv := control.StopUnixSocket()
	if errSrv != nil {
		log.Fatalf("Shutdown error: %v", errSrv)
	}
	if errControlSrv != nil {
		log.Fatalf("Control server shutdown error: %v", errControlSrv)
	}
	log.Println("Stopped")
}
