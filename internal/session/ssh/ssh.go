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

	"github.com/leofds/conduit/internal/resolver"
	"github.com/leofds/conduit/internal/session"
)

// Config holds session-level parameters for SSH connections.
type Config struct {
	IdleTimeout       time.Duration
	KeepaliveInterval time.Duration
	DialTimeout       time.Duration
	VerifyHostKey     bool               // enable TOFU host key verification
	TOFUAutoAccept    bool               // skip the interactive prompt and auto-accept unknown fingerprints
	KnownFingerprint  string             // expected SHA256 fingerprint; empty = first-use (TOFU)
	SaveHostKey       func(string) error // called on first use to persist the fingerprint; may be nil
}

type Runner struct {
	addr              string // host:port
	user              string
	pass              string
	privateKeyFile    string
	env               map[string]string
	term              string
	cols              uint16
	rows              uint16
	idleTimeout       time.Duration
	keepaliveInterval time.Duration
	dialTimeout       time.Duration
	verifyHostKey     bool
	tofuAutoAccept    bool
	knownFingerprint  string
	saveHostKey       func(string) error
}

func New(cfg resolver.SSHConfig, sshCfg Config, cols, rows uint16) *Runner {
	term := cfg.Term
	if term == "" {
		term = "xterm-256color"
	}
	if cols == 0 {
		cols = 80
	}
	if rows == 0 {
		rows = 24
	}
	return &Runner{
		addr:              fmt.Sprintf("%s:%s", cfg.Address, cfg.Port),
		user:              cfg.Username,
		pass:              cfg.Password,
		privateKeyFile:    cfg.PrivateKeyFile,
		env:               cfg.Env,
		term:              term,
		cols:              cols,
		rows:              rows,
		idleTimeout:       sshCfg.IdleTimeout,
		keepaliveInterval: sshCfg.KeepaliveInterval,
		dialTimeout:       sshCfg.DialTimeout,
		verifyHostKey:     sshCfg.VerifyHostKey,
		tofuAutoAccept:    sshCfg.TOFUAutoAccept,
		knownFingerprint:  sshCfg.KnownFingerprint,
		saveHostKey:       sshCfg.SaveHostKey,
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
		User: r.user,
		Auth: []gossh.AuthMethod{},
		BannerCallback: func(message string) error {
			normalized := strings.ReplaceAll(strings.ReplaceAll(message, "\r\n", "\n"), "\n", "\r\n")
			wsConn.WriteMessage(websocket.BinaryMessage, []byte(normalized)) //nolint:errcheck
			return nil
		},
		HostKeyCallback: gossh.InsecureIgnoreHostKey(), //nolint:gosec
		Timeout:         r.dialTimeout,
	}

	if r.verifyHostKey {
		cfg.HostKeyCallback = func(hostname string, _ net.Addr, key gossh.PublicKey) error {
			actual := gossh.FingerprintSHA256(key)
			if r.knownFingerprint == "" {
				if r.tofuAutoAccept {
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
				if r.saveHostKey != nil {
					if err := r.saveHostKey(actual); err != nil {
						log.Printf("SSH TOFU: failed to save fingerprint for %s: %v", hostname, err)
					}
				}
				return nil
			}
			if actual != r.knownFingerprint {
				return fmt.Errorf("host key mismatch: got %s, expected %s", actual, r.knownFingerprint)
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
	if r.privateKeyFile != "" {
		keyData, err := os.ReadFile(r.privateKeyFile)
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

	if r.pass != "" {
		// Automatic keyboard-interactive: silently answers all questions with the stored password.
		autoKbdInt := gossh.KeyboardInteractive(func(_, _ string, questions []string, _ []bool) ([]string, error) {
			answers := make([]string, len(questions))
			for i := range questions {
				answers[i] = r.pass
			}
			return answers, nil
		})
		authMethods = append(authMethods, gossh.Password(r.pass), autoKbdInt)
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
	if err := sesh.RequestPty(r.term, int(r.rows), int(r.cols), modes); err != nil {
		notify("PTY request failed: %v", err)
		return
	}

	for k, v := range r.env {
		if err := sesh.Setenv(k, v); err != nil {
			log.Printf("SSH setenv %s: %v (server may have rejected it)", k, err)
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
	if r.keepaliveInterval > 0 {
		go func() {
			ticker := time.NewTicker(r.keepaliveInterval)
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
	if r.idleTimeout > 0 {
		idleTimer = time.NewTimer(r.idleTimeout)
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
			idleTimer.Reset(r.idleTimeout)
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
