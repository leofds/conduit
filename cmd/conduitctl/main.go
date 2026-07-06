package main

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"strings"

	"github.com/leofds/conduit/internal/config"
)

func main() {
	cfg, err := config.Load(config.ConduitConfigPaths)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	socketPath := cfg.ControlSocket
	flag.StringVar(&socketPath, "socket", socketPath, "path to the conduit control socket")
	flag.Parse()

	if flag.NArg() == 0 {
		fmt.Fprintf(os.Stderr, "Usage: conduitctl [--socket <path>] <command> [args]\n\n")
		fmt.Fprintf(os.Stderr, "Commands:\n")
		fmt.Fprintf(os.Stderr, "  list          list active sessions\n")
		fmt.Fprintf(os.Stderr, "  close <id>    close a session by ID\n")
		fmt.Fprintf(os.Stderr, "  close-all     close all active sessions\n")
		os.Exit(1)
	}

	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		log.Fatalf("Failed to connect to control socket at %s: %v\n", socketPath, err)
	}

	defer func() { _ = conn.Close() }()

	command := strings.Join(flag.Args(), " ")
	if _, err := fmt.Fprintf(conn, "%s\n", command); err != nil {
		log.Fatalf("Failed to send command: %v", err)
	}

	// Read line by line until EOF (server closes connection).
	scanner := bufio.NewScanner(conn)
	for scanner.Scan() {
		fmt.Println(scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		log.Fatalf("Failed to read response: %v", err)
	}
}
