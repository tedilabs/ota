package overlay

import (
	"net/http"
	"sort"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/tedilabs/ota/internal/apilog"
	"github.com/tedilabs/ota/internal/tui/shared"
)

// APIRecorderPane indicates which pane currently owns focus. Tab /
// Shift-Tab cycle through Timeline → Request → Response → Timeline.
// Each pane carries its own BodyCursor so navigation, visual-line
// selection, and yank state stay independent across the three.
type APIRecorderPane int

const (
	APIRecorderPaneTimeline APIRecorderPane = iota
	APIRecorderPaneRequest
	APIRecorderPaneResponse
)

// APIRecorderModel renders the global "Okta API Timeline" overlay
// bound to the `~` keybinding. Three panes:
//
//   - Left  (40%): chronological timeline of captured round-trips —
//                  HH:MM:SS  METHOD  /path  status
//   - Right top  : REQUEST — method+url, headers, body
//   - Right bot. : RESPONSE — status+duration, headers, body, error
//
// Tab focuses the next pane; Shift-Tab the previous. Inside the
// focused pane:
//
//   - j/k · g/G — move cursor / top / bottom
//   - Ctrl-d/u  — half-page navigation
//   - v / V     — toggle visual-line selection
//   - y         — yank the cursor row (or visual range) to clipboard
//
// The overlay is purely a reader of an apilog.Recorder snapshot;
// data capture happens in the okta HTTP transport.
type APIRecorderModel struct {
	rec     *apilog.Recorder
	entries []apilog.Entry
	width   int
	height  int
	pane    APIRecorderPane

	// One BodyCursor per pane — Tab swaps focus without resetting
	// the others' cursors / visual state. The timeline cursor
	// addresses entry indices; the request / response cursors
	// address line indices in their respective bodies. When the
	// timeline cursor moves onto a different entry, the
	// request/response cursors are reset (Update tracks this via
	// SeqID before/after each key dispatch).
	timelineCursor shared.BodyCursor
	requestCursor  shared.BodyCursor
	responseCursor shared.BodyCursor
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
// snapshot. Newest first so operators land on the most recent entry.
func (m *APIRecorderModel) Refresh() {
	if m.rec == nil {
		m.entries = nil
		return
	}
	snap := m.rec.Snapshot()
	sort.Slice(snap, func(i, j int) bool {
		return snap[i].SeqID > snap[j].SeqID
	})
	m.entries = snap
	if m.timelineCursor.Line >= len(m.entries) {
		m.timelineCursor.Line = 0
		m.timelineCursor.CancelVisual()
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

// Update handles pane focus + per-pane navigation. The App Shell
// intercepts `~` and Esc at a higher level; everything else flows
// through here.
func (m APIRecorderModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	km, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}

	switch km.Type {
	case tea.KeyTab:
		m.pane = (m.pane + 1) % 3
		return m, nil
	case tea.KeyShiftTab:
		m.pane = (m.pane + 2) % 3 // -1 mod 3
		return m, nil
	}

	beforeSeq := m.cursorEntry().SeqID

	cur := m.activeCursor()
	lines := m.activeLines()
	viewport := m.activeViewportRows()
	total := len(lines)

	switch km.Type {
	case tea.KeyCtrlF:
		cur.PageDown(viewport, total)
	case tea.KeyCtrlB:
		cur.PageUp(viewport)
	case tea.KeyCtrlD:
		cur.HalfPageDown(viewport, total)
	case tea.KeyCtrlU:
		cur.HalfPageUp(viewport)
	case tea.KeyRunes:
		switch string(km.Runes) {
		case "j":
			cur.Down(total)
		case "k":
			cur.Up()
		case "g":
			cur.GoTop()
		case "G":
			cur.GoBottom(total)
		case "v", "V":
			if cur.Visual {
				cur.CancelVisual()
			} else {
				cur.StartVisual()
			}
		case "y":
			label := paneLabel(m.pane)
			m.writeCursor(*cur)
			return m, shared.YankCmd(*cur, lines, label)
		case "R":
			m.Refresh()
			return m, nil
		}
	}
	m.writeCursor(*cur)

	// If timeline navigation landed on a different entry, the
	// request/response panes' bodies just changed underneath their
	// cursors. Reset both so the operator doesn't see a stale
	// cursor pointing at line N of the previous entry's content.
	if afterSeq := m.cursorEntry().SeqID; afterSeq != beforeSeq {
		m.requestCursor = shared.BodyCursor{}
		m.responseCursor = shared.BodyCursor{}
	}
	return m, nil
}

// activeCursor returns a pointer to the focused pane's cursor so the
// switch above can mutate the right one in place.
func (m *APIRecorderModel) activeCursor() *shared.BodyCursor {
	switch m.pane {
	case APIRecorderPaneRequest:
		return &m.requestCursor
	case APIRecorderPaneResponse:
		return &m.responseCursor
	default:
		return &m.timelineCursor
	}
}

// writeCursor copies the modified cursor back onto the model — the
// switch in Update operated on a pointer, but Bubbletea's tea.Model
// contract is value-receiver, so we re-assign explicitly to keep the
// state visible to the next Update / View.
func (m *APIRecorderModel) writeCursor(c shared.BodyCursor) {
	switch m.pane {
	case APIRecorderPaneRequest:
		m.requestCursor = c
	case APIRecorderPaneResponse:
		m.responseCursor = c
	default:
		m.timelineCursor = c
	}
}

// activeLines returns the line slice that the focused pane navigates
// over. Timeline lines are pre-formatted entry rows; request /
// response lines come from the currently-selected entry.
func (m APIRecorderModel) activeLines() []string {
	switch m.pane {
	case APIRecorderPaneRequest:
		return buildRequestLines(m.cursorEntry())
	case APIRecorderPaneResponse:
		return buildResponseLines(m.cursorEntry())
	default:
		return m.timelineLines()
	}
}

// activeViewportRows reports how many rows the focused pane has on
// screen so PageDown / PageUp / EnsureVisible scroll by the correct
// amount.
func (m APIRecorderModel) activeViewportRows() int {
	_, _, _, _, _, _, paneH := m.layout()
	switch m.pane {
	case APIRecorderPaneTimeline:
		return paneH
	default:
		// Request and Response each get half of the right column,
		// minus the section header line.
		return paneH/2 - 1
	}
}

// cursorEntry returns the apilog.Entry the timeline cursor points
// at, or the zero-value entry when the timeline is empty.
func (m APIRecorderModel) cursorEntry() apilog.Entry {
	if len(m.entries) == 0 || m.timelineCursor.Line < 0 || m.timelineCursor.Line >= len(m.entries) {
		return apilog.Entry{}
	}
	return m.entries[m.timelineCursor.Line]
}

// timelineLines returns the timeline pane's rendered rows — used
// both by the View and by the BodyCursor when the timeline pane is
// focused (so j/k/g/G/v/V/y work uniformly).
func (m APIRecorderModel) timelineLines() []string {
	out := make([]string, len(m.entries))
	for i, e := range m.entries {
		out[i] = formatTimelineRow(e)
	}
	return out
}

// View composes the 3-pane layout into a modal box. Width comes
// from WithSize; falls back to 100 cells when unset.
func (m APIRecorderModel) View() string {
	tk := shared.PickTheme(shared.ResolveTheme(""))
	width, _, leftW, rightW, _, _, paneH := m.layout()

	timeline := m.renderTimeline(leftW, paneH, tk)
	right := m.renderRightColumn(rightW, paneH, tk)

	body := joinPanes(timeline, right, leftW, rightW, paneH, tk)

	footer := tk.Muted.Render(
		"Tab/Shift-Tab focus  ·  j/k g/G  ·  Ctrl-d/u  ·  v/V select  ·  y yank  ·  R refresh  ·  ~/Esc close",
	)

	title := "Okta API Timeline · " + paneLabel(m.pane) + " focused"
	if len(m.entries) > 0 {
		title += "  ·  " + strconv.Itoa(len(m.entries)) + " entries"
	}
	return shared.Modal(title, body+"\n"+footer, width)
}

// layout returns the geometry the View / activeViewportRows compute
// from. Returning a tuple keeps the math in one place — width clamp
// rules + panel split are centralized here.
func (m APIRecorderModel) layout() (width, height, leftW, rightW, innerW, innerH, paneH int) {
	width = m.width
	if width < 60 {
		width = 100
	}
	height = m.height
	if height < 12 {
		height = 24
	}
	innerW = width - 4
	innerH = height - 6
	if innerH < 8 {
		innerH = 8
	}
	leftW = innerW * 40 / 100
	if leftW < 30 {
		leftW = 30
	}
	rightW = innerW - leftW - 1
	if rightW < 30 {
		rightW = 30
	}
	paneH = innerH
	return
}

// renderTimeline returns the left pane's rendered rows — windowed
// around the timeline cursor and styled. When the timeline pane is
// focused the cursor row + visual range get the RowCursor highlight;
// otherwise the cursor is dimmed (Accent only) to indicate the
// inspected entry without drawing focus away from the active pane.
func (m APIRecorderModel) renderTimeline(width, height int, tk shared.Tokens) string {
	if len(m.entries) == 0 {
		return tk.Muted.Render(shared.PadOrTruncateVisible("(no API calls captured yet)", width))
	}
	// Window around the cursor.
	cursor := m.timelineCursor.Line
	top := cursor - height/2
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
	visualStart, visualEnd := m.timelineCursor.VisualRange()

	var b strings.Builder
	for i := top; i < end; i++ {
		row := formatTimelineRow(m.entries[i])
		row = shared.PadOrTruncateVisible(row, width)
		switch {
		case i == cursor && m.pane == APIRecorderPaneTimeline:
			row = tk.RowCursor.Render(shared.StripCSI(row))
		case m.pane == APIRecorderPaneTimeline && m.timelineCursor.Visual && i >= visualStart && i <= visualEnd:
			row = tk.RowCursor.Render(shared.StripCSI(row))
		case i == cursor:
			row = tk.Accent.Render(shared.StripCSI(row))
		default:
			row = colorMethodStatus(row, m.entries[i], tk)
		}
		b.WriteString(row)
		if i < end-1 {
			b.WriteByte('\n')
		}
	}
	return b.String()
}

// renderRightColumn stacks the REQUEST + RESPONSE boxes vertically.
// The focused pane (when one of req/resp) gets the active section
// header tint + body cursor + visual range; the other dims its
// header so the operator can tell at a glance which pane owns Tab.
func (m APIRecorderModel) renderRightColumn(width, height int, tk shared.Tokens) string {
	if len(m.entries) == 0 {
		empty := tk.Muted.Render(shared.PadOrTruncateVisible("(select a row)", width))
		return empty
	}
	half := height / 2
	if half < 4 {
		half = 4
	}
	reqLines := buildRequestLines(m.cursorEntry())
	respLines := buildResponseLines(m.cursorEntry())
	reqBox := m.renderPaneBox("REQUEST", APIRecorderPaneRequest,
		reqLines, m.requestCursor, width, half, tk)
	respBox := m.renderPaneBox("RESPONSE", APIRecorderPaneResponse,
		respLines, m.responseCursor, width, height-half, tk)
	return reqBox + "\n" + respBox
}

// renderPaneBox draws one labeled section with cursor + visual
// styling applied to the focused pane's lines. The pane header tint
// surfaces focus state at a glance.
func (m APIRecorderModel) renderPaneBox(label string, paneID APIRecorderPane, lines []string, cur shared.BodyCursor, width, height int, tk shared.Tokens) string {
	if width < 6 {
		width = 6
	}
	if height < 3 {
		height = 3
	}
	headerStyle := tk.Muted
	if m.pane == paneID {
		headerStyle = tk.Accent.Bold(true)
	}
	header := headerStyle.Render("── " + label + " ")
	header = header + tk.Muted.Render(strings.Repeat("─", maxInt(0, width-shared.VisibleWidth(header))))

	bodyHeight := height - 1
	cur.EnsureVisible(bodyHeight, len(lines))
	visualStart, visualEnd := cur.VisualRange()
	from := cur.Top
	if from < 0 {
		from = 0
	}
	if from > len(lines) {
		from = len(lines)
	}
	to := from + bodyHeight
	if to > len(lines) {
		to = len(lines)
	}

	out := []string{header}
	for i := 0; i < bodyHeight; i++ {
		idx := from + i
		if idx >= len(lines) {
			out = append(out, strings.Repeat(" ", width))
			continue
		}
		raw := lines[idx]
		prefix := "  "
		if m.pane == paneID && idx == cur.Line {
			prefix = "▸ "
		}
		full := shared.PadOrTruncateVisible(prefix+raw, width)
		switch {
		case m.pane == paneID && idx == cur.Line:
			full = tk.RowCursor.Render(shared.StripCSI(full))
		case m.pane == paneID && cur.Visual && idx >= visualStart && idx <= visualEnd:
			full = tk.RowCursor.Render(shared.StripCSI(full))
		}
		out = append(out, full)
	}
	return strings.Join(out, "\n")
}

// formatTimelineRow returns "HH:MM:SS  METHOD  /path  STATUS  120ms"
// — caller is responsible for padding/truncating to the pane width.
func formatTimelineRow(e apilog.Entry) string {
	stamp := e.Time.Local().Format("15:04:05")
	method := padRightAR(e.Method, 6)
	status := "    "
	if e.Status > 0 {
		status = strconv.Itoa(e.Status)
	} else if e.Err != "" {
		status = "ERR "
	}
	dur := strconv.FormatInt(e.DurationMS, 10) + "ms"
	return stamp + " " + method + " " + e.Path + "  " + status + "  " + tail(dur, 8)
}

// colorMethodStatus tints the row based on outcome — simpler than
// splicing per-token styles, and matches the list-row pattern.
func colorMethodStatus(row string, e apilog.Entry, tk shared.Tokens) string {
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

// buildRequestLines flattens the request fields into renderable
// lines. Stable order across renders so the request-pane cursor
// addresses the same content between movements.
func buildRequestLines(e apilog.Entry) []string {
	if e.SeqID == 0 && e.Method == "" {
		return []string{"(no entry selected)"}
	}
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
	if e.SeqID == 0 && e.Method == "" {
		return []string{"(no entry selected)"}
	}
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

// joinPanes splices the timeline (left) and right column line by
// line, with a 1-cell vertical gutter between.
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

// paneLabel surfaces the human-readable name for the focused pane in
// the modal title and yank toasts.
func paneLabel(p APIRecorderPane) string {
	switch p {
	case APIRecorderPaneRequest:
		return "Request"
	case APIRecorderPaneResponse:
		return "Response"
	default:
		return "Timeline"
	}
}

// SeqIDForCursor returns the SeqID at the timeline cursor (zero when
// nothing selected). Useful for stable test assertions.
func (m APIRecorderModel) SeqIDForCursor() uint64 {
	if m.timelineCursor.Line < 0 || m.timelineCursor.Line >= len(m.entries) {
		return 0
	}
	return m.entries[m.timelineCursor.Line].SeqID
}

// CursorEntry returns the entry at the timeline cursor (zero-value
// when out of range).
func (m APIRecorderModel) CursorEntry() apilog.Entry { return m.cursorEntry() }

// EntryCount surfaces the snapshot length so the App Shell's status
// line can advertise the timeline depth.
func (m APIRecorderModel) EntryCount() int { return len(m.entries) }

// FocusedPane reports which pane currently owns Tab focus — exposed
// for tests asserting the cycle wraps Timeline → Request → Response.
func (m APIRecorderModel) FocusedPane() APIRecorderPane { return m.pane }
