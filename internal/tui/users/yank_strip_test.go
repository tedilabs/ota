package users

// v0.2.1 #183: yank (`y`) on the User Detail body must write plain
// text to the clipboard. Previously the rendered lines carried
// inline ANSI escape codes (syntax highlighter colours, masked-line
// annotations) and those leaked into the operator's paste buffer
// verbatim — `\x1b[38;5;…m"login":\x1b[0m alice@acme.com`. Strip
// before clipboard write.

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/tedilabs/ota/internal/tui/shared"
)

// Test_DetailYank_StripsANSIBeforeClipboard verifies the strip
// happens against a known-styled string. The actual clipboard
// path is system-dependent (atotto/clipboard fails in CI without
// a display); we exercise the StripCSI codepath that the yank
// handler now applies before clipboard.WriteAll.
func Test_DetailYank_StripsANSIBeforeClipboard(t *testing.T) {
	t.Parallel()

	// Mimic a styled detail line as renderProfileTab / renderJSONTab
	// would produce — a key in tk.Header + a value in tk.Accent.
	styled := "\x1b[38;5;110;1m\"login\":\x1b[0m \x1b[38;5;75m\"alice@acme.com\"\x1b[0m"
	plain := shared.StripCSI(styled)
	assert.False(t, strings.Contains(plain, "\x1b["),
		"after StripCSI the string must not contain any CSI escape")
	assert.Equal(t, `"login": "alice@acme.com"`, plain,
		"StripCSI must preserve every visible glyph and only drop ANSI sequences")
}
