package overlay

import (
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/tedilabs/ota/internal/apilog"
	"github.com/tedilabs/ota/internal/tui/shared"
)

// APIRecorderPane indicates which pane currently owns the cursor —
// j/k navigates the timeline (Left) or scrolls the detail body
// (Right). Tab cycles between them.
type APIRecorderPane int

const (
	APIRecorderPaneTimeline APIRecorderPane = iota
	APIRecorderPaneDetail
)

// APIRecorderModel renders the global "Okta API Timeline" overlay
// bound to the `~` keybinding. Two panes:
//
//   - Left  (40%): chronological timeline of captured round-trips —
//                  HH:MM:SS  METHOD  /path  status
//   - Right (60%): split horizontally — top half shows the request
//                  (URL + headers + body), bottom half shows the
//                  response (status + headers + body)
//
// The overlay is purely a reader of an apilog.Recorder snapshot;
// data capture happens in the okta HTTP transport.
type APIRecorderModel struct {
	rec      *apilog.Recorder
	entries  []apilog.Entry
	cursor   int
	width    int
	height   int
	pane     APIRecorderPane
	detailScroll int
}

// NewAPIRecorderModel constructs the overlay model for the given
// recorder. Refresh() pulls a fresh snapshot from the recorder; the
// App Shell calls it whenever the overlay is opened so the timeline
// reflects the latest state.
func NewAPIRecorderModel(rec *apilog.Recorder) APIRecorderModel {
	m := APIRecorderModel{rec: rec}
	m.Refresh()
	return m
}

// Refresh rebuilds the entries slice from the recorder's current
// snapshot and snaps the cursor to the most recent row.
func (m *APIRecorderModel) Refresh() {
	if m.rec == nil {
		m.entries = nil
		return
	}
	snap := m.rec.Snapshot()
	// Newest first — operators usually want to inspect the most
	// recent call without scrolling.
	sort.Slice(snap, func(i, j int) bool {
		return snap[i].SeqID > snap[j].SeqID
	})
	m.entries = snap
	if m.cursor >= len(m.entries) {
		m.cursor = 0
	}
}

// WithSize sizes the overlay (App Shell pipes the chrome content
// rectangle in on every WindowSizeMsg).
func (m APIRecorderModel) WithSize(w, h int) APIRecorderModel {
	m.width = w
	m.height = h
	return m
}

// Init implements tea.Model.
func (m APIRecorderModel) Init() tea.Cmd { return nil }

// Update wires the overlay's local cursor + pane navigation. The
// App Shell intercepts `~` and Esc at a higher level; everything
// else flows through here.
func (m APIRecorderModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	km, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	switch km.Type {
	case tea.KeyTab:
		if m.pane == APIRecorderPaneTimeline {
			m.pane = APIRecorderPaneDetail
		} else {
			m.pane = APIRecorderPaneTimeline
		}
		return m, nil
	case tea.KeyShiftTab:
		if m.pane == APIRecorderPaneDetail {
			m.pane = APIRecorderPaneTimeline
		} else {
			m.pane = APIRecorderPaneDetail
		}
		return m, nil
	case tea.KeyRunes:
		switch string(km.Runes) {
		case "j":
			m.moveDown(1)
		case "k":
			m.moveUp(1)
		case "g":
			if m.pane == APIRecorderPaneTimeline {
				m.cursor = 0
			} else {
				m.detailScroll = 0
			}
		case "G":
			if m.pane == APIRecorderPaneTimeline && len(m.entries) > 0 {
				m.cursor = len(m.entries) - 1
			} else {
				// detail bottom — caller doesn't know body length;
				// approximate by a large step
				m.detailScroll += 1000
			}
		case "R":
			m.Refresh()
		}
	}
	return m, nil
}

func (m *APIRecorderModel) moveDown(n int) {
	if m.pane == APIRecorderPaneTimeline {
		if m.cursor+n < len(m.entries) {
			m.cursor += n
		} else if len(m.entries) > 0 {
			m.cursor = len(m.entries) - 1
		}
	} else {
		m.detailScroll += n
	}
}

func (m *APIRecorderModel) moveUp(n int) {
	if m.pane == APIRecorderPaneTimeline {
		if m.cursor-n >= 0 {
			m.cursor -= n
		} else {
			m.cursor = 0
		}
	} else {
		m.detailScroll -= n
		if m.detailScroll < 0 {
			m.detailScroll = 0
		}
	}
}

// View composes the 2-pane layout into a modal box. Width comes
// from WithSize; falls back to 100 cells when unset.
func (m APIRecorderModel) View() string {
	tk := shared.PickTheme(shared.ResolveTheme(""))
	width := m.width
	if width < 60 {
		width = 100
	}
	height := m.height
	if height < 12 {
		height = 24
	}

	innerWidth := width - 4 // modal borders + padding
	innerHeight := height - 6
	if innerHeight < 8 {
		innerHeight = 8
	}

	leftWidth := innerWidth * 40 / 100
	if leftWidth < 30 {
		leftWidth = 30
	}
	rightWidth := innerWidth - leftWidth - 1 // 1-cell gutter
	if rightWidth < 30 {
		rightWidth = 30
	}

	timeline := m.renderTimeline(leftWidth, innerHeight, tk)
	detail := m.renderDetail(rightWidth, innerHeight, tk)

	body := joinPanes(timeline, detail, leftWidth, rightWidth, innerHeight, tk)

	footer := tk.Muted.Render(
		"Tab focus  ·  j/k  ·  g/G  ·  R refresh  ·  ~ / Esc close",
	)

	title := "Okta API Timeline"
	if len(m.entries) > 0 {
		title += "  ·  " + strconv.Itoa(len(m.entries)) + " entries"
	}
	return shared.Modal(title, body+"\n"+footer, width)
}

// renderTimeline returns the left pane: a list of "HH:MM:SS METHOD
// /path STATUS" rows clipped to height. The cursor row gets the
// RowCursor highlight when the timeline pane is focused.
func (m APIRecorderModel) renderTimeline(width, height int, tk shared.Tokens) string {
	if len(m.entries) == 0 {
		return tk.Muted.Render(shared.PadOrTruncateVisible("(no API calls captured yet)", width))
	}
	// Window the slice around the cursor.
	top := m.cursor - height/2
	if top < 0 {
		top = 0
	}
	end := top + height
	if end > len(m.entries) {
		end = len(m.entries)
		top = end - height
		if top < 0 {
			top = 0
		}
	}
	var b strings.Builder
	for i := top; i < end; i++ {
		row := formatTimelineRow(m.entries[i], width)
		if i == m.cursor && m.pane == APIRecorderPaneTimeline {
			row = tk.RowCursor.Render(shared.StripCSI(row))
		} else if i == m.cursor {
			// Detail pane focused — keep the timeline cursor visible
			// but de-emphasized so the operator can tell which entry
			// is being inspected.
			row = tk.Accent.Render(shared.StripCSI(row))
		} else {
			row = colorMethodStatus(row, m.entries[i], tk)
		}
		b.WriteString(row)
		if i < end-1 {
			b.WriteByte('\n')
		}
	}
	return b.String()
}

// formatTimelineRow returns "HH:MM:SS  METHOD  /path  STATUS  120ms"
// padded / truncated to width.
func formatTimelineRow(e apilog.Entry, width int) string {
	stamp := e.Time.Local().Format("15:04:05")
	method := padRightAR(e.Method, 6)
	status := "    "
	if e.Status > 0 {
		status = strconv.Itoa(e.Status)
	} else if e.Err != "" {
		status = "ERR "
	}
	dur := strconv.FormatInt(e.DurationMS, 10) + "ms"
	row := stamp + " " + method + " " + e.Path + "  " + status + "  " + tail(dur, 8)
	return shared.PadOrTruncateVisible(row, width)
}

// colorMethodStatus tints the method (Accent) and status (Success /
// Warning / Danger) within an already-padded row.
func colorMethodStatus(row string, e apilog.Entry, tk shared.Tokens) string {
	// Apply tint to the entire row based on outcome — simpler than
	// splicing per-token styles, and matches the list-row pattern.
	switch {
	case e.Status >= 500:
		return tk.Danger.Render(row)
	case e.Status >= 400:
		return tk.Warning.Render(row)
	case e.Err != "":
		return tk.Danger.Render(row)
	default:
		return row
	}
}

// renderDetail returns the right pane: top half for the request,
// bottom half for the response.
func (m APIRecorderModel) renderDetail(width, height int, tk shared.Tokens) string {
	if len(m.entries) == 0 || m.cursor < 0 || m.cursor >= len(m.entries) {
		return tk.Muted.Render(shared.PadOrTruncateVisible("(select a row)", width))
	}
	e := m.entries[m.cursor]

	half := height / 2
	if half < 4 {
		half = 4
	}

	reqLines := buildRequestLines(e)
	respLines := buildResponseLines(e)

	if m.pane == APIRecorderPaneDetail && m.detailScroll > 0 {
		// Apply scroll to the combined section (request first, then
		// response). detailScroll counts whole lines.
		combined := append([]string{}, reqLines...)
		combined = append(combined, "")
		combined = append(combined, respLines...)
		if m.detailScroll >= len(combined) {
			m.detailScroll = len(combined) - 1
		}
		combined = combined[m.detailScroll:]
		// Re-split request/response by the marker we just inserted.
		reqLines, respLines = splitOnBlank(combined)
	}

	reqBox := boxify("REQUEST", reqLines, width, half, tk)
	respBox := boxify("RESPONSE", respLines, width, height-half, tk)
	return reqBox + "\n" + respBox
}

// buildRequestLines flattens the request fields into renderable
// lines: method+url, headers, body.
func buildRequestLines(e apilog.Entry) []string {
	var out []string
	out = append(out, e.Method+"  "+e.URL)
	out = append(out, "")
	out = append(out, headerLines(e.RequestHeaders)...)
	if e.RequestBody != "" {
		out = append(out, "")
		out = append(out, "Body:")
		out = append(out, splitLines(e.RequestBody)...)
	}
	return out
}

// buildResponseLines flattens the response fields: status, duration,
// headers, body, error.
func buildResponseLines(e apilog.Entry) []string {
	var out []string
	statusLine := strconv.Itoa(e.Status) + "  ·  " +
		strconv.FormatInt(e.DurationMS, 10) + "ms"
	if e.Err != "" {
		statusLine += "  ·  " + e.Err
	}
	out = append(out, statusLine)
	out = append(out, "")
	out = append(out, headerLines(e.ResponseHeaders)...)
	if e.ResponseBody != "" {
		out = append(out, "")
		out = append(out, "Body:")
		out = append(out, splitLines(e.ResponseBody)...)
	}
	return out
}

// headerLines returns "Key: value" lines sorted by key.
func headerLines(h http.Header) []string {
	if len(h) == 0 {
		return []string{"(no headers)"}
	}
	keys := make([]string, 0, len(h))
	for k := range h {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := make([]string, 0, len(h))
	for _, k := range keys {
		for _, v := range h[k] {
			out = append(out, k+": "+v)
		}
	}
	return out
}

// boxify wraps lines in a labeled bordered region of (width, height).
func boxify(label string, lines []string, width, height int, tk shared.Tokens) string {
	if width < 6 {
		width = 6
	}
	if height < 3 {
		height = 3
	}
	header := tk.Accent.Bold(true).Render("── " + label + " ")
	header = header + tk.Muted.Render(strings.Repeat("─", maxInt(0, width-shared.VisibleWidth(header))))
	bodyHeight := height - 1
	out := []string{header}
	for i := 0; i < bodyHeight; i++ {
		if i < len(lines) {
			out = append(out, shared.PadOrTruncateVisible(lines[i], width))
		} else {
			out = append(out, strings.Repeat(" ", width))
		}
	}
	return strings.Join(out, "\n")
}

// joinPanes splices the timeline (left) and detail (right) panes
// side-by-side, line by line, with a 1-cell vertical gutter between.
func joinPanes(left, right string, leftW, rightW, height int, tk shared.Tokens) string {
	leftLines := strings.Split(left, "\n")
	rightLines := strings.Split(right, "\n")
	gutter := tk.Muted.Render("│")
	out := make([]string, height)
	for i := 0; i < height; i++ {
		l := ""
		if i < len(leftLines) {
			l = leftLines[i]
		}
		l = shared.PadOrTruncateVisible(l, leftW)
		r := ""
		if i < len(rightLines) {
			r = rightLines[i]
		}
		r = shared.PadOrTruncateVisible(r, rightW)
		out[i] = l + gutter + r
	}
	return strings.Join(out, "\n")
}

// splitLines splits a string into individual lines without a final
// empty trailing entry.
func splitLines(s string) []string {
	return strings.Split(strings.TrimRight(s, "\n"), "\n")
}

// splitOnBlank splits the combined request/response slice back into
// two halves at the first empty string marker — mirrors the marker
// renderDetail inserts before scrolling.
func splitOnBlank(combined []string) (req, resp []string) {
	for i, ln := range combined {
		if ln == "" {
			return combined[:i], combined[i+1:]
		}
	}
	return combined, nil
}

// padRightAR pads s to n cells with trailing spaces.
func padRightAR(s string, n int) string {
	w := shared.VisibleWidth(s)
	if w >= n {
		return s
	}
	return s + strings.Repeat(" ", n-w)
}

// tail returns the last n cells of s, padding from the left when
// shorter.
func tail(s string, n int) string {
	if shared.VisibleWidth(s) >= n {
		return s
	}
	return strings.Repeat(" ", n-shared.VisibleWidth(s)) + s
}

// maxInt returns the larger of a, b.
func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// SeqIDForCursor returns the SeqID at the current cursor position
// (zero when there's nothing selected). Useful for stable test
// assertions.
func (m APIRecorderModel) SeqIDForCursor() uint64 {
	if m.cursor < 0 || m.cursor >= len(m.entries) {
		return 0
	}
	return m.entries[m.cursor].SeqID
}

// CursorEntry returns the entry at the current cursor (zero-value
// when out of range).
func (m APIRecorderModel) CursorEntry() apilog.Entry {
	if m.cursor < 0 || m.cursor >= len(m.entries) {
		return apilog.Entry{}
	}
	return m.entries[m.cursor]
}

// EntryCount surfaces the snapshot length so the App Shell's status
// line can advertise the timeline depth.
func (m APIRecorderModel) EntryCount() int { return len(m.entries) }

var _ = time.Now // ensure time package stays imported even if unused
