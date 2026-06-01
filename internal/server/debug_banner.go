package server

import (
	"fmt"

	"github.com/gorilla/websocket"

	"github.com/leofds/conduit/internal/config"
	"github.com/leofds/conduit/internal/session/local"
	"github.com/leofds/conduit/internal/session/ssh"
	"github.com/leofds/conduit/internal/version"
)

func (s *Server) writeDebugBanner(wsConn *websocket.Conn, host string, cfg any) {
	write := func(line string) {
		_ = wsConn.WriteMessage(websocket.BinaryMessage, []byte(line))
	}

	blue := "\x1b[34m"
	reset := "\x1b[0m"
	write("\r\n")
	write(blue + "  ██████╗ ██████╗ ███╗   ██╗██████╗ ██╗   ██╗██╗████████╗\r\n" + reset)
	write(blue + " ██╔════╝██╔═══██╗████╗  ██║██╔══██╗██║   ██║██║╚══██╔══╝\r\n" + reset)
	write(blue + " ██║     ██║   ██║██╔██╗ ██║██║  ██║██║   ██║██║   ██║\r\n" + reset)
	write(blue + " ██║     ██║   ██║██║╚██╗██║██║  ██║██║   ██║██║   ██║\r\n" + reset)
	write(blue + " ╚██████╗╚██████╔╝██║ ╚████║██████╔╝╚██████╔╝██║   ██║\r\n" + reset)
	write(blue + "  ╚═════╝ ╚═════╝ ╚═╝  ╚═══╝╚═════╝  ╚═════╝ ╚═╝   ╚═╝\r\n" + reset)
	write("\r\n")
	write("Project: https://github.com/leofds/conduit\r\n")
	write(fmt.Sprintf("Version: %s\r\n", version.Version))
	write("License: MIT\r\n")
	write("---------------------------------------------------------\r\n")

	sessionType := "ssh"
	if host == config.Local {
		sessionType = "local"
	}
	write(fmt.Sprintf("Session method: %s\r\n", sessionType))
	write(fmt.Sprintf("Host: %s\r\n", host))
	write("---------------------------------------------------------\r\n")

	switch sess := cfg.(type) {
	case local.Config:
		write(fmt.Sprintf("Command: %s\r\n", sess.Command))
		write(fmt.Sprintf("Term: %s\r\n", sess.Term))
		write(fmt.Sprintf("Working dir: %s\r\n", sess.WorkingDir))
		write(fmt.Sprintf("Idle timeout: %s\r\n", sess.IdleTimeout))
		write("Env:\r\n")
		for k, v := range sess.Env {
			write(fmt.Sprintf("  %s=%s\r\n", k, v))
		}
		write("---------------------------------------------------------\r\n")
		write("Starting local session...\r\n\r\n")
	case ssh.Config:
		write(fmt.Sprintf("Address: %s\r\n", sess.Address))
		write(fmt.Sprintf("Port: %s\r\n", sess.Port))
		write(fmt.Sprintf("Username: %s\r\n", sess.Username))
		write(fmt.Sprintf("Term: %s\r\n", sess.Term))
		write(fmt.Sprintf("Idle timeout: %s\r\n", sess.IdleTimeout))
		write(fmt.Sprintf("Keepalive interval: %s\r\n", sess.KeepaliveInterval))
		write(fmt.Sprintf("Dial timeout: %s\r\n", sess.DialTimeout))
		write(fmt.Sprintf("Verify host key: %t\r\n", sess.VerifyHostKey))
		write(fmt.Sprintf("TOFU auto accept: %t\r\n", sess.TOFUAutoAccept))
		write(fmt.Sprintf("Auth: %s\r\n", sshAuthMethod(sess)))
		write("Env:\r\n")
		for k, v := range sess.Env {
			write(fmt.Sprintf("  %s=%s\r\n", k, v))
		}
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
