package shared

import (
	"strings"

	"github.com/tedilabs/ota/internal/okta/errormap"
)

// ErrorPanel renders a list-screen inline error panel (TUI_DESIGN §17.1).
// Layout: a "[X] <heading>" line, the user-friendly message from
// errormap.UserMessage(err), and an action hint footer. The panel is wrapped
// in the standard Modal box so it stands out inside the chrome.
//
// resource is the noun shown in the heading ("users", "groups", etc.). When
// err is nil the panel is empty (caller should not invoke ErrorPanel).
func ErrorPanel(resource string, err error) string {
	if err == nil {
		return ""
	}
	heading := "[X] Failed to load " + resource
	body := heading + "\n\n" +
		"    " + errormap.UserMessage(err) + "\n\n" +
		"<R> retry     <:about> token info     <:errors> history"
	return Modal("Error · "+resource, body, 70)
}

// ErrorBanner renders a one-line banner suitable for partial failures (e.g.,
// a tab that failed while the rest of the screen succeeded; TUI_DESIGN
// §17.1). Returns the formatted line without trailing newline.
func ErrorBanner(message string) string {
	return "[!] " + strings.TrimSpace(message)
}
