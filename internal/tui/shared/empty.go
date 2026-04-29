package shared

// EmptyState helpers — v0.2.0 unified placeholder rendering for the
// "no rows yet" surfaces scattered across the codebase: User Detail
// Groups/Apps lazy fetch, Group Detail Members tab, Policy Detail
// Rules tab, factors lists, etc.
//
// Each helper returns a Muted-styled single line ready to drop into
// a scroll box or tab body. Centralising the styling keeps the cue
// language consistent — operators learn one phrasing pattern and
// see the same Muted tone across every screen.

import (
	"errors"
	"fmt"
)

// LoadingRow renders the universal "loading <label>…" placeholder
// shown while a lazy fetch is in flight. label is the singular noun
// matching the data being loaded ("members", "rules", "groups").
func LoadingRow(label string, tk Tokens) string {
	return tk.Muted.Render("loading " + label + "…")
}

// EmptyRow renders the "(no <label>)" placeholder shown when a
// fetch resolved with zero rows. The parens distinguish "we
// looked, found nothing" from "still loading" / "fetch failed".
func EmptyRow(label string, tk Tokens) string {
	return tk.Muted.Render("(no " + label + ")")
}

// ErrorRow renders a single-line error placeholder for inline
// surfaces (boxes / tabs / extras). Falls back to the source label
// when err is nil so callers don't need to special-case the
// argument. Use shared.ErrorPanel for full-body error rendering.
func ErrorRow(source string, err error, tk Tokens) string {
	if err == nil {
		return tk.Danger.Render(fmt.Sprintf("error: %s", source))
	}
	if source == "" {
		return tk.Danger.Render("error: " + err.Error())
	}
	return tk.Danger.Render(fmt.Sprintf("error %s: %s", source, err.Error()))
}

// PlaceholderRow returns the appropriate single-line placeholder
// for an inline data surface based on its current state. Use this
// when a screen consumes (loaded, err, items) and wants the same
// "loading… → (no foo) / error" decision applied everywhere.
//
// Returns "" when items > 0 — caller renders the real rows.
func PlaceholderRow(loaded bool, err error, items int, label string, tk Tokens) string {
	switch {
	case err != nil:
		return ErrorRow(label, err, tk)
	case !loaded:
		return LoadingRow(label, tk)
	case items == 0:
		return EmptyRow(label, tk)
	}
	return ""
}

// ensures errors is referenced in this file (some callers will use
// errors.Is via this package).
var _ = errors.New