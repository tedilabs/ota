// Package admins renders the Administrators list / detail surface
// (read-only — flat (user, role) rows from /api/v1/iam/assignees/users
// fanned out per-user via /api/v1/users/{id}/roles).
package admins

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
	Port            domain.AdministratorsPort
	Clock           clock.Clock
	Logger          *slog.Logger
	Keys            keys.ResolvedMap
	Width           int
	Height          int
	RefreshInterval time.Duration
	Initial         []domain.Administrator
}

func New(deps Deps) simpleres.Model[domain.Administrator] {
	return simpleres.New[domain.Administrator](simpleres.Deps[domain.Administrator]{
		Spec:            spec(deps.Port),
		Clock:           deps.Clock,
		Logger:          deps.Logger,
		Keys:            deps.Keys,
		Width:           deps.Width,
		Height:          deps.Height,
		RefreshInterval: deps.RefreshInterval,
		Initial:         deps.Initial,
	})
}

func spec(port domain.AdministratorsPort) simpleres.Spec[domain.Administrator] {
	return simpleres.Spec[domain.Administrator]{
		Title: "Administrators",
		List: func(ctx context.Context) ([]domain.Administrator, error) {
			if port == nil {
				return nil, nil
			}
			return port.List(ctx)
		},
		// (UserID, RoleID) is the natural key — a single user with two
		// admin roles produces two rows that must dedupe distinctly.
		ID: func(a domain.Administrator) string { return a.UserID + "/" + a.RoleID },
		FilterMatch: func(a domain.Administrator, needle string) bool {
			return strings.Contains(strings.ToLower(a.Login), needle) ||
				strings.Contains(strings.ToLower(a.RoleType), needle) ||
				strings.Contains(strings.ToLower(a.RoleLabel), needle)
		},
		Columns: []simpleres.Column[domain.Administrator]{
			{Header: "LOGIN", Width: 28, Format: func(a domain.Administrator) string { return a.Login }},
			{Header: "ROLE", Width: 26, Format: func(a domain.Administrator) string { return a.RoleType }},
			{Header: "LABEL", Flex: true, Format: func(a domain.Administrator) string { return a.RoleLabel }},
			{Header: "STATUS", Width: 9, Format: func(a domain.Administrator) string {
				if a.Status == "" {
					return "—"
				}
				return a.Status
			}},
		},
		Pretty: func(a domain.Administrator) string {
			const w = 14
			var b strings.Builder
			b.WriteString(shared.KVRow("login", a.Login, w))
			b.WriteByte('\n')
			b.WriteString(shared.KVRow("userId", a.UserID, w))
			b.WriteByte('\n')
			b.WriteString(shared.KVRow("firstName", a.FirstName, w))
			b.WriteByte('\n')
			b.WriteString(shared.KVRow("lastName", a.LastName, w))
			b.WriteByte('\n')
			b.WriteString(shared.KVRow("roleId", a.RoleID, w))
			b.WriteByte('\n')
			b.WriteString(shared.KVRow("roleType", a.RoleType, w))
			b.WriteByte('\n')
			b.WriteString(shared.KVRow("roleLabel", a.RoleLabel, w))
			b.WriteByte('\n')
			b.WriteString(shared.KVRow("status", a.Status, w))
			return b.String()
		},
	}
}

var _ = time.Now // kept for future column callbacks needing a clock
