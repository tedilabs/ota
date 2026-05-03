// Package apitokens renders the Okta API Tokens list / detail
// surface (read-only — minting / revoking is out of scope).
package apitokens

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
	Port            domain.APITokensPort
	Clock           clock.Clock
	Logger          *slog.Logger
	Keys            keys.ResolvedMap
	Width           int
	Height          int
	RefreshInterval time.Duration
	Initial         []domain.APIToken
}

func New(deps Deps) simpleres.Model[domain.APIToken] {
	clk := deps.Clock
	if clk == nil {
		clk = clock.Real()
	}
	return simpleres.New[domain.APIToken](simpleres.Deps[domain.APIToken]{
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

func spec(port domain.APITokensPort, clk clock.Clock) simpleres.Spec[domain.APIToken] {
	return simpleres.Spec[domain.APIToken]{
		Title: "API Tokens",
		List: func(ctx context.Context) ([]domain.APIToken, error) {
			if port == nil {
				return nil, nil
			}
			return port.List(ctx)
		},
		ID: func(t domain.APIToken) string { return t.ID },
		FilterMatch: func(t domain.APIToken, needle string) bool {
			return strings.Contains(strings.ToLower(t.Name), needle) ||
				strings.Contains(strings.ToLower(t.ClientName), needle) ||
				strings.Contains(strings.ToLower(t.UserID), needle)
		},
		Columns: []simpleres.Column[domain.APIToken]{
			{Header: "NAME", Width: 28, Format: func(t domain.APIToken) string { return t.Name }},
			{Header: "OWNER", Width: 22, Format: func(t domain.APIToken) string { return t.UserID }},
			{Header: "CLIENT", Flex: true, Format: func(t domain.APIToken) string { return t.ClientName }},
			{Header: "EXPIRES", Width: 12, Format: func(t domain.APIToken) string {
				now := clk.Now()
				if t.ExpiresAt.IsZero() {
					return "never"
				}
				return shared.RelativeTime(&t.ExpiresAt, now)
			}},
			{Header: "UPDATED", Width: 12, Format: func(t domain.APIToken) string {
				now := clk.Now()
				if t.LastUpdated.IsZero() {
					return "—"
				}
				return shared.RelativeTime(&t.LastUpdated, now)
			}},
		},
		Pretty: func(t domain.APIToken) string {
			const w = 14
			var b strings.Builder
			b.WriteString(shared.KVRow("id", t.ID, w))
			b.WriteByte('\n')
			b.WriteString(shared.KVRow("name", t.Name, w))
			b.WriteByte('\n')
			b.WriteString(shared.KVRow("userId", t.UserID, w))
			b.WriteByte('\n')
			b.WriteString(shared.KVRow("clientName", t.ClientName, w))
			b.WriteByte('\n')
			b.WriteString(shared.KVRow("created", t.Created.Format(time.RFC3339), w))
			b.WriteByte('\n')
			b.WriteString(shared.KVRow("lastUpdated", t.LastUpdated.Format(time.RFC3339), w))
			b.WriteByte('\n')
			expires := "never"
			if !t.ExpiresAt.IsZero() {
				expires = t.ExpiresAt.Format(time.RFC3339)
			}
			b.WriteString(shared.KVRow("expiresAt", expires, w))
			return b.String()
		},
	}
}
