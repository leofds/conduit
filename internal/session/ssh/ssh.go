package ssh

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"os"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	gossh "golang.org/x/crypto/ssh"

	"github.com/leofds/conduit/internal/session"
)

// Config holds session-level parameters for SSH connections.
type Config struct {
	Address           string             // host or IP address
	Port              string             // TCP port
	Username          string             // SSH username
	Password          string             // SSH password (optional if using key-based auth)
	PrivateKeyFile    string             // path to private key file (optional if using password)
	Env               map[string]string  // environment variables to set in the session
	Term              string             // terminal type (e.g. "xterm-256color")
	IdleTimeout       time.Duration      // duration of inactivity before auto-closing the session; 0 = no timeout
	KeepaliveInterval time.Duration      // interval for sending SSH keepalive messages; 0 = disable
	DialTimeout       time.Duration      // timeout for establishing the SSH connection
	VerifyHostKey     bool               // enable TOFU host key verification
	AutoAcceptHostKey bool               // skip the interactive prompt and auto-accept unknown fingerprints
	KnownFingerprint  string             // expected SHA256 fingerprint; empty = first-use (TOFU)
	SaveHostKey       func(string) error // called on first use to persist the fingerprint; may be nil
	DebugBanner       bool               // if true, rejected env vars are shown in the terminal
}

type Runner struct {
	addr string
	cfg  Config
	rows uint16
	cols uint16
}

func New(sshCfg Config, cols, rows uint16) *Runner {
	return &Runner{
		addr: fmt.Sprintf("%s:%s", sshCfg.Address, sshCfg.Port),
		cfg:  sshCfg,
		cols: cols,
		rows: rows,
	}
}

func (r *Runner) Run(parentCtx context.Context, wsConn *websocket.Conn) {
	ctx, cancel := context.WithCancel(parentCtx)
	defer cancel()

	notify := func(format string, args ...any) {
		msg := fmt.Sprintf("\r\n["+format+"]\r\n", args...)
		log.Printf("%s", msg)
		wsConn.WriteMessage(websocket.BinaryMessage, []byte(msg)) //nolint:errcheck
		cancel()
	}

	readLine := func() (string, error) {
		return session.ReadLine(wsConn)
	}

	cfg := &gossh.ClientConfig{
		User: r.cfg.Username,
		Auth: []gossh.AuthMethod{},
		BannerCallback: func(message string) error {
			normalized := strings.ReplaceAll(strings.ReplaceAll(message, "\r\n", "\n"), "\n", "\r\n")
			wsConn.WriteMessage(websocket.BinaryMessage, []byte(normalized)) //nolint:errcheck
			return nil
		},
		HostKeyCallback: gossh.InsecureIgnoreHostKey(), //nolint:gosec
		Timeout:         r.cfg.DialTimeout,
	}

	if r.cfg.VerifyHostKey {
		cfg.HostKeyCallback = func(hostname string, _ net.Addr, key gossh.PublicKey) error {
			actual := gossh.FingerprintSHA256(key)
			if r.cfg.KnownFingerprint == "" {
				if r.cfg.AutoAcceptHostKey {
					// Auto-accept: trust and persist without prompting.
					log.Printf("SSH TOFU: auto-accepting %s with fingerprint %s", hostname, actual)
				} else {
					// Interactive: ask the user to confirm the fingerprint.
					prompt := fmt.Sprintf(
						"\r\nThe authenticity of host '\x1b[92m%s\x1b[0m' can't be established.\r\nKey fingerprint is \x1b[96m%s\x1b[0m.\r\nAre you sure you want to continue connecting? [yes/no]: ",
						hostname, actual,
					)
					wsConn.WriteMessage(websocket.BinaryMessage, []byte(prompt)) //nolint:errcheck
					answer, err := session.ReadLineEcho(wsConn)
					if err != nil {
						return fmt.Errorf("failed to read user confirmation: %w", err)
					}
					wsConn.WriteMessage(websocket.BinaryMessage, []byte("\r\n\r\n")) //nolint:errcheck
					if strings.ToLower(strings.TrimSpace(answer)) != "yes" {
						return fmt.Errorf("host key not trusted by user")
					}
					log.Printf("SSH TOFU: trusting %s with fingerprint %s", hostname, actual)
				}
				if r.cfg.SaveHostKey != nil {
					if err := r.cfg.SaveHostKey(actual); err != nil {
						log.Printf("SSH TOFU: failed to save fingerprint for %s: %v", hostname, err)
					}
				}
				return nil
			}
			if actual != r.cfg.KnownFingerprint {
				return fmt.Errorf("host key mismatch: got %s, expected %s", actual, r.cfg.KnownFingerprint)
			}
			return nil
		}
	}

	// Interactive keyboard-interactive: forwards questions to the browser and waits for user input.
	kbdInt := gossh.KeyboardInteractive(func(name, instruction string, questions []string, echos []bool) ([]string, error) {
		answers := make([]string, len(questions))
		if name != "" {
			wsConn.WriteMessage(websocket.BinaryMessage, []byte(name+"\r\n")) //nolint:errcheck
		}
		if instruction != "" {
			wsConn.WriteMessage(websocket.BinaryMessage, []byte(instruction+"\r\n")) //nolint:errcheck
		}
		for i, q := range questions {
			if q != "" {
				wsConn.WriteMessage(websocket.BinaryMessage, []byte(q)) //nolint:errcheck
			}
			answer, err := readLine()
			if err != nil {
				return nil, err
			}
			wsConn.WriteMessage(websocket.BinaryMessage, []byte("\r\n")) //nolint:errcheck
			answers[i] = answer
		}
		return answers, nil
	})

	passwordPrompt := func() (string, error) {
		wsConn.WriteMessage(websocket.BinaryMessage, []byte("Password: ")) //nolint:errcheck
		pass, err := readLine()
		if err != nil {
			return "", err
		}
		wsConn.WriteMessage(websocket.BinaryMessage, []byte("\r\n")) //nolint:errcheck
		return pass, nil
	}

	var authMethods []gossh.AuthMethod

	// Key-based auth: read the private key file and use it if provided.
	if r.cfg.PrivateKeyFile != "" {
		keyData, err := os.ReadFile(r.cfg.PrivateKeyFile)
		if err != nil {
			notify("SSH key file read failed: %v", err)
			return
		}
		signer, err := gossh.ParsePrivateKey(keyData)
		if err != nil {
			notify("SSH key parse failed: %v", err)
			return
		}
		authMethods = append(authMethods, gossh.PublicKeys(signer))
	}

	if r.cfg.Password != "" {
		// Automatic keyboard-interactive: silently answers all questions with the stored password.
		autoKbdInt := gossh.KeyboardInteractive(func(_, _ string, questions []string, _ []bool) ([]string, error) {
			answers := make([]string, len(questions))
			for i := range questions {
				answers[i] = r.cfg.Password
			}
			return answers, nil
		})
		authMethods = append(authMethods, gossh.Password(r.cfg.Password), autoKbdInt)
	}

	if len(authMethods) == 0 {
		// No credentials configured: fall back to interactive prompts.
		authMethods = []gossh.AuthMethod{
			gossh.RetryableAuthMethod(kbdInt, 3),
			gossh.RetryableAuthMethod(gossh.PasswordCallback(passwordPrompt), 3),
		}
	}

	cfg.Auth = authMethods

	client, err := gossh.Dial("tcp", r.addr, cfg)
	if err != nil {
		if strings.Contains(err.Error(), "host key mismatch") {
			notify("SSH host key mismatch: possible security issue or changed server key")
		} else if strings.Contains(err.Error(), "unable to authenticate") {
			notify("SSH authentication failed: wrong credentials")
		} else {
			notify("SSH connection failed: %v", err)
		}
		return
	}
	defer func() { _ = client.Close() }()

	sesh, err := client.NewSession()
	if err != nil {
		notify("SSH session failed: %v", err)
		return
	}
	defer func() { _ = sesh.Close() }()

	modes := gossh.TerminalModes{
		gossh.ECHO:          1,
		gossh.TTY_OP_ISPEED: 14400,
		gossh.TTY_OP_OSPEED: 14400,
	}
	if err := sesh.RequestPty(r.cfg.Term, int(r.rows), int(r.cols), modes); err != nil {
		notify("PTY request failed: %v", err)
		return
	}

	for k, v := range r.cfg.Env {
		if err := sesh.Setenv(k, v); err != nil {
			log.Printf("SSH setenv %s: %v (server may have rejected it)", k, err)
			if r.cfg.DebugBanner {
				wsConn.WriteMessage(websocket.BinaryMessage, []byte(fmt.Sprintf("\x1b[31mEnv rejected: %s=%s\x1b[0m\r\n", k, v))) //nolint:errcheck
			}
		}
	}

	writer := &session.Writer{Conn: wsConn}
	sesh.Stdout = writer
	sesh.Stderr = writer

	stdinPipe, err := sesh.StdinPipe()
	if err != nil {
		notify("stdin pipe failed: %v", err)
		return
	}
	defer func() { _ = stdinPipe.Close() }()

	if err := sesh.Shell(); err != nil {
		notify("shell start failed: %v", err)
		return
	}

	// Keepalive goroutine
	if r.cfg.KeepaliveInterval > 0 {
		go func() {
			ticker := time.NewTicker(r.cfg.KeepaliveInterval)
			defer ticker.Stop()
			for {
				select {
				case <-ticker.C:
					if _, _, err := client.SendRequest("keepalive@openssh.com", true, nil); err != nil {
						notify("SSH keepalive failed — connection lost")
						return
					}
				case <-ctx.Done():
					return
				}
			}
		}()
	}

	// Session-exit watcher
	go func() {
		if err := sesh.Wait(); err != nil {
			notify("session ended: %v", err)
		} else {
			notify("session ended")
		}
	}()

	// Idle timer
	var idleTimer *time.Timer
	if r.cfg.IdleTimeout > 0 {
		idleTimer = time.NewTimer(r.cfg.IdleTimeout)
		defer idleTimer.Stop()
		go func() {
			select {
			case <-idleTimer.C:
				notify("session closed due to inactivity")
			case <-ctx.Done():
			}
		}()
	}

	// Main loop: WebSocket -> SSH stdin
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		msgType, data, err := wsConn.ReadMessage()
		if err != nil {
			return
		}

		if idleTimer != nil {
			if !idleTimer.Stop() {
				select {
				case <-idleTimer.C:
				default:
				}
			}
			idleTimer.Reset(r.cfg.IdleTimeout)
		}

		if msgType == websocket.TextMessage {
			var msg session.ResizeMsg
			if err := json.Unmarshal(data, &msg); err == nil && msg.Type == "resize" {
				sesh.WindowChange(int(msg.Rows), int(msg.Cols)) //nolint:errcheck
				continue
			}
		}

		if _, err := stdinPipe.Write(data); err != nil {
			return
		}
	}
}
