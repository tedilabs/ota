// Package authservers renders the Custom Authorization Servers list
// / detail surface (read-only).
package authservers

import (
	"context"
	"log/slog"
	"strings"
	"time"

	"github.com/tedilabs/ota/internal/clock"
	"github.com/tedilabs/ota/internal/domain"
	"github.com/tedilabs/ota/internal/keys"
	"github.com/tedilabs/ota/internal/tui/shared"
	"github.com/tedilabs/ota/internal/tui/simpleres"
)

type Deps struct {
	Port            domain.AuthorizationServersPort
	Clock           clock.Clock
	Logger          *slog.Logger
	Keys            keys.ResolvedMap
	Width           int
	Height          int
	RefreshInterval time.Duration
	Initial         []domain.AuthorizationServer
}

func New(deps Deps) simpleres.Model[domain.AuthorizationServer] {
	clk := deps.Clock
	if clk == nil {
		clk = clock.Real()
	}
	return simpleres.New[domain.AuthorizationServer](simpleres.Deps[domain.AuthorizationServer]{
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

func spec(port domain.AuthorizationServersPort, clk clock.Clock) simpleres.Spec[domain.AuthorizationServer] {
	return simpleres.Spec[domain.AuthorizationServer]{
		Title: "Authorization Servers",
		List: func(ctx context.Context) ([]domain.AuthorizationServer, error) {
			if port == nil {
				return nil, nil
			}
			return port.List(ctx)
		},
		ID: func(s domain.AuthorizationServer) string { return s.ID },
		FilterMatch: func(s domain.AuthorizationServer, needle string) bool {
			return strings.Contains(strings.ToLower(s.Name), needle) ||
				strings.Contains(strings.ToLower(s.Issuer), needle) ||
				strings.Contains(strings.ToLower(s.Description), needle)
		},
		Columns: []simpleres.Column[domain.AuthorizationServer]{
			{Header: "STATUS", Width: 9, Format: func(s domain.AuthorizationServer) string { return string(s.Status) }},
			{Header: "NAME", Width: 24, Format: func(s domain.AuthorizationServer) string { return s.Name }},
			{Header: "ISSUER", Flex: true, Format: func(s domain.AuthorizationServer) string { return s.Issuer }},
			{Header: "AUDIENCES", Width: 14, Format: func(s domain.AuthorizationServer) string {
				return strings.Join(s.Audiences, ",")
			}},
			{Header: "UPDATED", Width: 12, Format: func(s domain.AuthorizationServer) string {
				now := clk.Now()
				if s.LastUpdated.IsZero() {
					return "—"
				}
				return shared.RelativeTime(&s.LastUpdated, now)
			}},
		},
		Pretty: func(s domain.AuthorizationServer) string {
			const w = 14
			var b strings.Builder
			b.WriteString(shared.KVRow("id", s.ID, w))
			b.WriteByte('\n')
			b.WriteString(shared.KVRow("name", s.Name, w))
			b.WriteByte('\n')
			b.WriteString(shared.KVRow("description", s.Description, w))
			b.WriteByte('\n')
			b.WriteString(shared.KVRow("issuer", s.Issuer, w))
			b.WriteByte('\n')
			b.WriteString(shared.KVRow("audiences", strings.Join(s.Audiences, ", "), w))
			b.WriteByte('\n')
			b.WriteString(shared.KVRow("status", string(s.Status), w))
			b.WriteByte('\n')
			b.WriteString(shared.KVRow("created", s.Created.Format(time.RFC3339), w))
			b.WriteByte('\n')
			b.WriteString(shared.KVRow("lastUpdated", s.LastUpdated.Format(time.RFC3339), w))
			return b.String()
		},
	}
}
