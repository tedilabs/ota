// Package zones renders the Network Zones list/detail surface
// (read-only). Built on simpleres so the navigation / filter / detail-
// tab semantics match every other resource without duplicating the
// boilerplate.
package zones

import (
	"context"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/tedilabs/ota/internal/clock"
	"github.com/tedilabs/ota/internal/domain"
	"github.com/tedilabs/ota/internal/keys"
	"github.com/tedilabs/ota/internal/tui/shared"
	"github.com/tedilabs/ota/internal/tui/simpleres"
)

// Deps wraps simpleres.Deps with Okta-specific port wiring.
type Deps struct {
	Port            domain.NetworkZonesPort
	Clock           clock.Clock
	Logger          *slog.Logger
	Keys            keys.ResolvedMap
	Width           int
	Height          int
	RefreshInterval time.Duration
	Initial         []domain.NetworkZone
}

// New constructs the Network Zones list/detail model.
func New(deps Deps) simpleres.Model[domain.NetworkZone] {
	clk := deps.Clock
	if clk == nil {
		clk = clock.Real()
	}
	return simpleres.New[domain.NetworkZone](simpleres.Deps[domain.NetworkZone]{
		Spec:            spec(deps.Port, clk),
		Clock:           deps.Clock,
		Logger:          deps.Logger,
		Keys:            deps.Keys,
		Width:           deps.Width,
		Height:          deps.Height,
		RefreshInterval: deps.RefreshInterval,
		Initial:         deps.Initial,
	})
}

func spec(port domain.NetworkZonesPort, clk clock.Clock) simpleres.Spec[domain.NetworkZone] {
	return simpleres.Spec[domain.NetworkZone]{
		Title: "Network Zones",
		List: func(ctx context.Context) ([]domain.NetworkZone, error) {
			if port == nil {
				return nil, nil
			}
			return port.List(ctx)
		},
		ID: func(z domain.NetworkZone) string { return z.ID },
		FilterMatch: func(z domain.NetworkZone, needle string) bool {
			return strings.Contains(strings.ToLower(z.Name), needle) ||
				strings.Contains(strings.ToLower(string(z.Type)), needle) ||
				strings.Contains(strings.ToLower(string(z.Usage)), needle)
		},
		Columns: []simpleres.Column[domain.NetworkZone]{
			{Header: "TYPE", Width: 8, Format: func(z domain.NetworkZone) string { return string(z.Type) }},
			{Header: "STATUS", Width: 9, Format: func(z domain.NetworkZone) string { return string(z.Status) }},
			{Header: "USAGE", Width: 11, Format: func(z domain.NetworkZone) string { return string(z.Usage) }},
			{Header: "SYSTEM", Width: 7, Format: func(z domain.NetworkZone) string {
				if z.System {
					return "[SYS]"
				}
				return "-"
			}},
			{Header: "NAME", Flex: true, Format: func(z domain.NetworkZone) string { return z.Name }},
			{Header: "UPDATED", Width: 12, Format: func(z domain.NetworkZone) string {
				now := clk.Now()
				return relTime(z.LastUpdated, now)
			}},
		},
		Pretty: func(z domain.NetworkZone) string {
			const w = 14
			var b strings.Builder
			b.WriteString(shared.KVRow("id", z.ID, w))
			b.WriteByte('\n')
			b.WriteString(shared.KVRow("name", z.Name, w))
			b.WriteByte('\n')
			b.WriteString(shared.KVRow("type", string(z.Type), w))
			b.WriteByte('\n')
			b.WriteString(shared.KVRow("status", string(z.Status), w))
			b.WriteByte('\n')
			b.WriteString(shared.KVRow("usage", string(z.Usage), w))
			b.WriteByte('\n')
			b.WriteString(shared.KVRow("system", strconv.FormatBool(z.System), w))
			b.WriteByte('\n')
			b.WriteString(shared.KVRow("created", z.Created.Format(time.RFC3339), w))
			b.WriteByte('\n')
			b.WriteString(shared.KVRow("lastUpdated", z.LastUpdated.Format(time.RFC3339), w))
			return b.String()
		},
	}
}

// relTime renders a relative-time stamp; "—" for zero values.
func relTime(t time.Time, now time.Time) string {
	if t.IsZero() {
		return "—"
	}
	return shared.RelativeTime(&t, now)
}
