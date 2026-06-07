package server

import (
	"fmt"

	"github.com/gorilla/websocket"

	"github.com/leofds/conduit/internal/config"
	"github.com/leofds/conduit/internal/session/local"
	"github.com/leofds/conduit/internal/session/ssh"
	"github.com/leofds/conduit/internal/version"
)

func (s *Server) writeDebugBanner(wsConn *websocket.Conn, host string, cfg any, cols uint16) {
	write := func(line string) {
		_ = wsConn.WriteMessage(websocket.BinaryMessage, []byte(line))
	}

	blue := "\x1b[34m"
	valueColor := "\x1b[33m"
	reset := "\x1b[0m"
	write("\r\n")
	write(blue + "  ██████╗ ██████╗ ███╗   ██╗██████╗ ██╗   ██╗██╗████████╗\r\n" + reset)
	write(blue + " ██╔════╝██╔═══██╗████╗  ██║██╔══██╗██║   ██║██║╚══██╔══╝\r\n" + reset)
	write(blue + " ██║     ██║   ██║██╔██╗ ██║██║  ██║██║   ██║██║   ██║\r\n" + reset)
	write(blue + " ██║     ██║   ██║██║╚██╗██║██║  ██║██║   ██║██║   ██║\r\n" + reset)
	write(blue + " ╚██████╗╚██████╔╝██║ ╚████║██████╔╝╚██████╔╝██║   ██║\r\n" + reset)
	write(blue + "  ╚═════╝ ╚═════╝ ╚═╝  ╚═══╝╚═════╝  ╚═════╝ ╚═╝   ╚═╝\r\n" + reset)
	write("\r\n")
	write(fmt.Sprintf("Project: %s%s%s\r\n", valueColor, "https://github.com/leofds/conduit", reset))
	write(fmt.Sprintf("Version: %s%s%s\r\n", valueColor, version.Version, reset))
	write(fmt.Sprintf("License: %s%s%s\r\n", valueColor, "MIT", reset))

	// Session details
	write("---------------------------------------------------------\r\n")
	sessionType := "ssh"
	if host == config.Local {
		sessionType = "local"
	}
	write(fmt.Sprintf("Session method: %s%s%s\r\n", valueColor, sessionType, reset))
	write(fmt.Sprintf("Host: %s%s%s\r\n", valueColor, host, reset))

	// HTTP headers
	write("---------------------------------------------------------\r\n")
	write("HTTP headers:\r\n")
	for k, v := range s.httpHeaders {
		write(fmt.Sprintf("  %s: %s%s%s\r\n", k, valueColor, v, reset))
	}

	// Session configuration
	write("---------------------------------------------------------\r\n")
	switch sess := cfg.(type) {
	case local.Config:
		write(fmt.Sprintf("Command: %s%s%s\r\n", valueColor, sess.Command, reset))
		write(fmt.Sprintf("Term: %s%s%s\r\n", valueColor, sess.Term, reset))
		write(fmt.Sprintf("Working dir: %s%s%s\r\n", valueColor, sess.WorkingDir, reset))
		write(fmt.Sprintf("Idle timeout: %s%s%s\r\n", valueColor, sess.IdleTimeout, reset))
		write("Env: ")
		first := true
		for k, v := range sess.Env {
			if !first {
				write("  ")
			}
			first = false
			write(fmt.Sprintf("%s=%s%s%s", k, valueColor, v, reset))
		}
		write("\r\n")
		write("---------------------------------------------------------\r\n")
		write("Starting local session...\r\n\r\n")
	case ssh.Config:
		write(fmt.Sprintf("Address: %s%s%s\r\n", valueColor, sess.Address, reset))
		write(fmt.Sprintf("Port: %s%s%s\r\n", valueColor, sess.Port, reset))
		write(fmt.Sprintf("Username: %s%s%s\r\n", valueColor, sess.Username, reset))
		write(fmt.Sprintf("Auth: %s%s%s\r\n", valueColor, sshAuthMethod(sess), reset))
		write(fmt.Sprintf("Term: %s%s%s\r\n", valueColor, sess.Term, reset))
		write(fmt.Sprintf("Idle timeout: %s%s%s\r\n", valueColor, sess.IdleTimeout, reset))
		write(fmt.Sprintf("Keepalive interval: %s%s%s\r\n", valueColor, sess.KeepaliveInterval, reset))
		write(fmt.Sprintf("Dial timeout: %s%s%s\r\n", valueColor, sess.DialTimeout, reset))
		write(fmt.Sprintf("Verify host key: %s%t%s\r\n", valueColor, sess.VerifyHostKey, reset))
		write(fmt.Sprintf("Auto accept host key: %s%t%s\r\n", valueColor, sess.AutoAcceptHostKey, reset))
		write("Env: ")
		first := true
		for k, v := range sess.Env {
			if !first {
				write("  ")
			}
			first = false
			write(fmt.Sprintf("%s=%s%s%s", k, valueColor, v, reset))
		}
		write("\r\n")
		write("---------------------------------------------------------\r\n")
		write("Starting SSH session...\r\n\r\n")
	default:
		return
	}
}

func sshAuthMethod(cfg ssh.Config) string {
	if cfg.PrivateKeyFile != "" {
		return "private key"
	}
	if cfg.Password != "" {
		return "password"
	}
	return "interactive"
}
