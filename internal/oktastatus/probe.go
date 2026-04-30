// Package oktastatus polls https://status.okta.com/api/v2/status.json
// and exposes the most recent snapshot for the chrome's title bar.
//
// The endpoint is Statuspage.io's standard component-status feed
// (Okta hosts on Statuspage). Anonymous, rate-limit-free, and shape:
//
//	{
//	  "page": { "name": "Okta", … },
//	  "status": {
//	    "indicator":   "none" | "minor" | "major" | "critical" | "maintenance",
//	    "description": "All Systems Operational" | "Partial Outage" | …
//	  }
//	}
//
// We only consume the `status` block; the page metadata is ignored.
package oktastatus

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// DefaultEndpoint is Okta's public status feed.
const DefaultEndpoint = "https://status.okta.com/api/v2/status.json"

// Indicator classifies the overall service state. Maps directly to
// Statuspage.io's documented values; unknown / unreachable states
// collapse to IndicatorUnknown so the badge falls back to a muted
// glyph instead of misleading the operator.
type Indicator int

const (
	IndicatorUnknown     Indicator = iota // probe hasn't returned yet, or HTTP error
	IndicatorOperational                  // "none" — all green
	IndicatorMinor                        // partial / minor disruption
	IndicatorMajor                        // major outage
	IndicatorCritical                     // critical outage
	IndicatorMaintenance                  // scheduled maintenance window
)

// Snapshot is one fetch result, suitable for direct rendering.
// Description is Okta's free-text summary; the chrome formats it
// alongside the emoji so the operator reads both at a glance.
type Snapshot struct {
	Indicator   Indicator
	Description string
	FetchedAt   time.Time
}

// Probe is a thin client over the status endpoint. The default zero
// value is usable: it will hit DefaultEndpoint with a 5s context.
type Probe struct {
	Endpoint string        // override for tests
	Client   *http.Client  // override for tests
	Timeout  time.Duration // per-request; defaults 5s
}

// Fetch issues one GET against the endpoint and returns a Snapshot.
// On any error the snapshot's Indicator is IndicatorUnknown — the
// chrome renders a muted glyph instead of crashing the boot path.
func (p *Probe) Fetch(ctx context.Context) Snapshot {
	endpoint := p.Endpoint
	if endpoint == "" {
		endpoint = DefaultEndpoint
	}
	timeout := p.Timeout
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	cli := p.Client
	if cli == nil {
		cli = &http.Client{Timeout: timeout}
	}

	cctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(cctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return Snapshot{Indicator: IndicatorUnknown, Description: err.Error(), FetchedAt: time.Now()}
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "ota/0.2.2 (+okta-status)")

	resp, err := cli.Do(req)
	if err != nil {
		return Snapshot{Indicator: IndicatorUnknown, Description: err.Error(), FetchedAt: time.Now()}
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return Snapshot{
			Indicator:   IndicatorUnknown,
			Description: fmt.Sprintf("status %d", resp.StatusCode),
			FetchedAt:   time.Now(),
		}
	}

	var body struct {
		Status struct {
			Indicator   string `json:"indicator"`
			Description string `json:"description"`
		} `json:"status"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return Snapshot{Indicator: IndicatorUnknown, Description: "decode: " + err.Error(), FetchedAt: time.Now()}
	}
	return Snapshot{
		Indicator:   indicatorFromString(body.Status.Indicator),
		Description: body.Status.Description,
		FetchedAt:   time.Now(),
	}
}

func indicatorFromString(s string) Indicator {
	switch s {
	case "none":
		return IndicatorOperational
	case "minor":
		return IndicatorMinor
	case "major":
		return IndicatorMajor
	case "critical":
		return IndicatorCritical
	case "maintenance":
		return IndicatorMaintenance
	}
	return IndicatorUnknown
}

// Emoji returns the cue glyph for the chrome title bar. Plain ASCII
// fallback (for NO_COLOR / monochrome) is the first letter so the
// segment never disappears entirely on terminals without emoji.
func (i Indicator) Emoji() string {
	switch i {
	case IndicatorOperational:
		return "🟢"
	case IndicatorMinor:
		return "🟡"
	case IndicatorMajor:
		return "🟠"
	case IndicatorCritical:
		return "🔴"
	case IndicatorMaintenance:
		return "🛠"
	}
	return "❔"
}

// Label returns the short status word — paired with the emoji on the
// title bar. Distinct from Snapshot.Description (free text from Okta)
// because the description varies in length and would push the title
// bar's right segment off-screen.
func (i Indicator) Label() string {
	switch i {
	case IndicatorOperational:
		return "ok"
	case IndicatorMinor:
		return "minor"
	case IndicatorMajor:
		return "major"
	case IndicatorCritical:
		return "critical"
	case IndicatorMaintenance:
		return "maint"
	}
	return "?"
}