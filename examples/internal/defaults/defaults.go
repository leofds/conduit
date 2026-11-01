// Package defaults provides a ready-to-use conduit.Options pre-filled with the
// same sensible defaults that the conduit binary applies when no conduit.yaml
// is present. It lets the examples focus on wiring a Resolver and a known-hosts
// store instead of repeating every timeout and option by hand.
//
// It mirrors internal/config defaultConfig() but is built on the public
// conduit.Options type, so external code can copy this pattern freely.
package defaults

import (
	"time"

	conduit "github.com/leofds/conduit"
)

// Options returns a conduit.Options populated with the standard defaults.
// The caller MUST still set Resolver (required) and may override any field.
func Options() conduit.Options {
	return conduit.Options{
		// Behaviour flags (match internal/config defaults).
		AllowLocal:  true,
		Demo:        true,
		DebugBanner: true,

		// KnownHosts:     nil, // nil → in-memory TOFU store (examples set this explicitly)
		// RequestMapper:  nil, // nil → host from URL path, token from cookie/Bearer header
		// AllowedOrigins: nil, // nil → allow all origins; []string{} would BLOCK all origins

		// Empty map → no extra HTTP response headers.
		HTTPHeaders: map[string]string{},

		// Generic terminal theming/scrollback passed to xterm.js.
		TerminalOptions: map[string]any{
			"scrollback": 5000,
			"theme": map[string]any{
				"background": "#1e1e1e",
				"foreground": "#d4d4d4",
			},
		},

		// HTTP server timeouts.
		Timeouts: conduit.Timeouts{
			Read:        10 * time.Second,
			Write:       0,
			ReadHeader:  10 * time.Second,
			Idle:        120 * time.Second,
			WSHandshake: 10 * time.Second,
		},

		// Global defaults for resolved SSH sessions.
		SSH: conduit.SSHOptions{
			Port:              "22",
			Term:              "xterm-256color",
			IdleTimeout:       10 * time.Minute,
			KeepaliveInterval: 30 * time.Second,
			DialTimeout:       10 * time.Second,
			VerifyHostKey:     true,
			AutoAcceptHostKey: false,
			Env:               map[string]string{},
		},

		// Global defaults for resolved local sessions.
		Local: conduit.LocalOptions{
			Command:     "/bin/bash",
			Term:        "xterm-256color",
			WorkingDir:  "", // inherit the process working directory
			IdleTimeout: 10 * time.Minute,
			Env:         map[string]string{},
		},
	}
}
