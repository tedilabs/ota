package users_test

// Pins the abnormal-status row tint (issue #155). The tint shows up
// at runtime as a colour bg; tests run with NO_COLOR=1 so lipgloss
// strips ANSI entirely. We assert behaviour at the mapping layer
// (shared.RowStyleForStatus → expected bucket) rather than at the
// rendered-byte layer where NO_COLOR makes everything plain.

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/tedilabs/ota/internal/tui/shared"
)

func Test_RowStyleForStatus_AbnormalGetTint(t *testing.T) {
	t.Parallel()

	tk := shared.Dark()

	// Each abnormal status must produce a tint style.
	for _, status := range []string{
		"LOCKED_OUT", "INVALID",
		"SUSPENDED", "PASSWORD_EXPIRED",
		"DEPROVISIONED", "INACTIVE",
	} {
		_, ok := shared.RowStyleForStatus(status, tk)
		assert.Truef(t, ok,
			"abnormal status %q must map to a row tint style (issue #155)", status)
	}

	// Benign / transient statuses must NOT be tinted.
	for _, status := range []string{
		"ACTIVE", "STAGED", "PROVISIONED", "",
	} {
		_, ok := shared.RowStyleForStatus(status, tk)
		assert.Falsef(t, ok,
			"benign status %q must NOT carry a row tint", status)
	}
}
