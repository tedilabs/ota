// Package main is ota's entrypoint. Responsibilities: flag parsing, config
// load, dependency assembly (via wire.go), and handing off to tea.Program.
//
// No business logic lives here. See docs/ARCHITECTURE.md §4.1 & §10.1.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	rtdebug "runtime/debug"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"

	"github.com/tedilabs/ota/internal/app"
	"github.com/tedilabs/ota/internal/logger"
	"github.com/tedilabs/ota/internal/tui/shared"
	"github.com/tedilabs/ota/internal/version"
)

func main() {
	defer scrubbedRecover(os.Stderr)
	code := run(os.Args[1:], os.Stdout, os.Stderr)
	os.Exit(code)
}

// scrubbedRecover catches any panic that escapes run() and prints the panic
// value plus stack trace through logger.ScrubText so SSWS / Authorization /
// api_token fragments never reach stderr or core dumps (REQ-C05 AC-3,
// QA-008).
func scrubbedRecover(w *os.File) {
	r := recover()
	if r == nil {
		return
	}
	msg := fmt.Sprintf("ota: panic: %v\n\n%s", r, rtdebug.Stack())
	fmt.Fprintln(w, logger.ScrubText(msg))
	os.Exit(2)
}

// run parses flags, wires dependencies, and launches the Bubbletea program.
// It returns the process exit code. Streams are injected so tests can capture
// --help / -version output.
func run(args []string, stdout, stderr *os.File) int {
	fs := flag.NewFlagSet("ota", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var (
		configPath  = fs.String("config", "", "explicit path to config.yaml (overrides XDG)")
		profile     = fs.String("profile", "", "name of the active tenant profile")
		tokenEnv    = fs.String("token-env", "", "env variable name to read the API token from")
		debugMode   = fs.Bool("debug", false, "enable debug logging to ~/.cache/ota/debug.log")
		pollSec     = fs.Int("poll-interval", 0, "override Logs tail poll interval in seconds")
		showVersion = fs.Bool("version", false, "print version and exit")
	)
	if err := fs.Parse(args); err != nil {
		// flag.ErrHelp is not an error — user asked for help.
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		fmt.Fprintln(stderr, "ota:", err)
		return 2
	}

	if *showVersion {
		fmt.Fprintf(stdout, "ota %s (commit %s, built %s)\n", version.Tag, version.Commit, version.BuildTime)
		return 0
	}

	// Pin the rendering profile up front so monochrome environments (NO_COLOR
	// or piped stdout) skip ANSI emission entirely. lipgloss otherwise
	// auto-detects from termenv at first Style.Render — too late if early
	// chrome rendering has already started.
	if shared.MonochromeEnabled() {
		lipgloss.SetColorProfile(termenv.Ascii)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	wireModel, _, err := Wire(ctx, WireInput{
		ConfigPath: *configPath,
		Profile:    *profile,
		TokenEnv:   *tokenEnv,
		Debug:      *debugMode,
		PollSec:    *pollSec,
	})

	var rootModel tea.Model = wireModel
	if err != nil {
		// Boot failures (missing profile, unreadable config, invalid token URL)
		// used to print to stderr and exit. The chrome-styled boot error screen
		// surfaces the same message in-app so the operator can read it without
		// relying on terminal scrollback (TUI_DESIGN §17.1, REQ-C04 AC-4).
		rootModel = app.NewBootErrorModel(err, bootHint(*profile))
		fmt.Fprintln(stderr, "ota:", logger.ScrubText(err.Error()))
	}

	prog := tea.NewProgram(rootModel, tea.WithAltScreen())
	if _, runErr := prog.Run(); runErr != nil {
		fmt.Fprintln(stderr, "ota: tea program:", logger.ScrubText(runErr.Error()))
		return 1
	}
	if err != nil {
		return 1
	}
	return 0
}

// bootHint builds a short remediation hint based on which inputs were
// missing. Surfaced by BootErrorModel below the user message.
func bootHint(profile string) string {
	parts := []string{"Configure ~/.config/ota/config.yaml"}
	if profile == "" {
		parts = append(parts, "or pass --profile <name>")
	} else {
		parts = append(parts, "or check that profile "+profile+" is defined")
	}
	parts = append(parts, "and set OKTA_API_TOKEN in your environment")
	return strings.Join(parts, " ") + "."
}

// errWireNotReady indicates that MVP wiring cannot complete in the absence
// of a valid profile + token.
var errWireNotReady = errors.New("ota: a profile with a reachable API token is required")
