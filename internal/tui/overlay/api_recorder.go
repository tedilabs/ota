package overlay

import (
	"net/http"
	"sort"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

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
// focused (so j/k/g/G/v/V/y work uniformly). Yanked rows carry the
// raw cell text, no ANSI styling.
func (m APIRecorderModel) timelineLines() []string {
	out := make([]string, len(m.entries))
	pathWidth := m.timelinePathWidth()
	for i, e := range m.entries {
		out[i] = formatTimelineRow(e, pathWidth)
	}
	return out
}

// timelinePathWidth returns the cells available for the PATH column
// after subtracting the fixed columns + gutters. Mirrors the layout
// math used by the header + render so they stay in lockstep.
func (m APIRecorderModel) timelinePathWidth() int {
	_, _, leftW, _, _, _, _ := m.layout()
	return timelinePathWidthFor(leftW)
}

// Column widths for the timeline. Tuned so 3-digit status codes +
// "DELETE" methods + "120ms"-class durations all align.
const (
	tlColTime     = 8 // HH:MM:SS
	tlColMethod   = 7 // DELETE + space
	tlColStatus   = 5 // " 200 " padded badge
	tlColDuration = 8 // "999ms  " right-aligned
	tlGutter      = 1 // 1-cell gutter between columns
)

// timelinePathWidthFor returns the dynamic PATH column width given
// the timeline pane's total width. Caller passes the pane width so
// the same formula drives header + data row layouts.
func timelinePathWidthFor(paneWidth int) int {
	const fixed = tlColTime + tlColMethod + tlColStatus + tlColDuration + 4*tlGutter
	w := paneWidth - fixed
	if w < 8 {
		w = 8
	}
	return w
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

// renderTimeline returns the left pane's rendered rows — column
// header at the top, windowed data rows below. When the timeline
// pane is focused the cursor row + visual range get the RowCursor
// highlight; otherwise the cursor stays Accent-tinted so the
// operator can still see which entry the request/response panes are
// reflecting. Status cells carry a per-class background tint
// (2xx green / 4xx yellow / 5xx + ERR red) so the eye snaps to
// non-2xx outcomes at a glance.
func (m APIRecorderModel) renderTimeline(width, height int, tk shared.Tokens) string {
	pathWidth := timelinePathWidthFor(width)
	header := timelineHeader(pathWidth, tk)
	headerWidth := shared.VisibleWidth(header)
	if headerWidth < width {
		header = header + strings.Repeat(" ", width-headerWidth)
	}

	if len(m.entries) == 0 {
		empty := tk.Muted.Render(shared.PadOrTruncateVisible("(no API calls captured yet)", width))
		return header + "\n" + empty
	}
	// Reserve 1 row for the header — the data window scrolls under it.
	dataHeight := height - 1
	if dataHeight < 1 {
		dataHeight = 1
	}
	cursor := m.timelineCursor.Line
	top := cursor - dataHeight/2
	if top < 0 {
		top = 0
	}
	end := top + dataHeight
	if end > len(m.entries) {
		end = len(m.entries)
		top = end - dataHeight
		if top < 0 {
			top = 0
		}
	}
	visualStart, visualEnd := m.timelineCursor.VisualRange()

	var b strings.Builder
	b.WriteString(header)
	b.WriteByte('\n')
	for i := top; i < end; i++ {
		entry := m.entries[i]
		var row string
		switch {
		case i == cursor && m.pane == APIRecorderPaneTimeline:
			row = tk.RowCursor.Render(shared.StripCSI(formatTimelineRow(entry, pathWidth)))
			row = shared.PadOrTruncateVisible(row, width)
		case m.pane == APIRecorderPaneTimeline && m.timelineCursor.Visual && i >= visualStart && i <= visualEnd:
			row = tk.RowCursor.Render(shared.StripCSI(formatTimelineRow(entry, pathWidth)))
			row = shared.PadOrTruncateVisible(row, width)
		case i == cursor:
			row = tk.Accent.Render(shared.StripCSI(formatTimelineRow(entry, pathWidth)))
			row = shared.PadOrTruncateVisible(row, width)
		default:
			row = renderTimelineRowStyled(entry, pathWidth, tk)
			row = padTrailing(row, width)
		}
		b.WriteString(row)
		if i < end-1 {
			b.WriteByte('\n')
		}
	}
	return b.String()
}

// timelineHeader returns the column-header row matching the data
// rows' layout. Header text is muted + bold so it reads as chrome,
// not data.
func timelineHeader(pathWidth int, tk shared.Tokens) string {
	g := strings.Repeat(" ", tlGutter)
	header := padRightAR("WHEN", tlColTime) + g +
		padRightAR("METHOD", tlColMethod) + g +
		padRightAR("PATH", pathWidth) + g +
		padRightAR("STATUS", tlColStatus) + g +
		padLeftAR("MS", tlColDuration)
	return tk.Muted.Bold(true).Render(header)
}

// renderTimelineRowStyled returns one data row with per-cell
// styling: muted timestamp, accent method, plain path, status badge
// tinted by class, muted duration. Cells are padded to the canonical
// column widths defined above so the header lines up with every row.
func renderTimelineRowStyled(e apilog.Entry, pathWidth int, tk shared.Tokens) string {
	g := strings.Repeat(" ", tlGutter)
	stamp := tk.Muted.Render(padRightAR(e.Time.Local().Format("15:04:05"), tlColTime))
	method := methodStyle(e.Method, tk).Render(padRightAR(e.Method, tlColMethod))
	path := shared.PadOrTruncateVisible(e.Path, pathWidth)
	status := statusBadge(e, tk)
	dur := tk.Muted.Render(padLeftAR(formatDurationMS(e.DurationMS), tlColDuration))
	return stamp + g + method + g + path + g + status + g + dur
}

// methodStyle picks a foreground tint for the HTTP method cell. GET
// and HEAD read as routine reads; mutating verbs (POST/PUT/PATCH/
// DELETE) get warmer colors so destructive ops stand out in a long
// timeline.
func methodStyle(method string, tk shared.Tokens) lipgloss.Style {
	switch method {
	case "GET", "HEAD":
		return tk.Accent
	case "POST", "PUT", "PATCH":
		return tk.Warning
	case "DELETE":
		return tk.Danger
	default:
		return tk.FG
	}
}

// statusBadge renders the status column cell with a per-class
// background highlight: 2xx green, 3xx neutral (muted), 4xx yellow,
// 5xx + transport errors red. Cell width is tlColStatus (5) so the
// header aligns. Empty status (in-flight) renders as a muted dash.
func statusBadge(e apilog.Entry, tk shared.Tokens) string {
	const w = tlColStatus
	if e.Err != "" {
		return tk.BadgeUnmask.Render(centerAR("ERR", w))
	}
	if e.Status <= 0 {
		return tk.Muted.Render(centerAR("---", w))
	}
	code := strconv.Itoa(e.Status)
	switch {
	case e.Status >= 500:
		return tk.BadgeUnmask.Render(centerAR(code, w))
	case e.Status >= 400:
		return tk.BadgeLarge.Render(centerAR(code, w))
	case e.Status >= 200 && e.Status < 300:
		return tk.BadgeRule.Render(centerAR(code, w))
	default:
		// 3xx / 1xx — no background, just plain text. Keeps the
		// neutral case from competing with the alarm classes.
		return centerAR(code, w)
	}
}

// formatDurationMS renders a millisecond count as "Nms" / "1.2s".
// Right-aligned by the caller via padLeftAR.
func formatDurationMS(ms int64) string {
	if ms < 1000 {
		return strconv.FormatInt(ms, 10) + "ms"
	}
	// Show 1 decimal on seconds so "1.2s" stays terse.
	tenths := ms / 100
	whole := tenths / 10
	frac := tenths % 10
	return strconv.FormatInt(whole, 10) + "." + strconv.FormatInt(frac, 10) + "s"
}

// renderRightColumn stacks the REQUEST + RESPONSE boxes vertically.
// The focused pane (when one of req/resp) gets the active section
// header tint + body cursor + visual range; the other dims its
// header so the operator can tell at a glance which pane owns Tab.
//
// Each pane keeps both a plain and a styled view of its lines:
// plainLines drive cursor accounting + yank (so clipboard text is
// pristine), styledLines feed the screen render so JSON bodies +
// header keys + status badges all carry their syntax tints.
func (m APIRecorderModel) renderRightColumn(width, height int, tk shared.Tokens) string {
	if len(m.entries) == 0 {
		empty := tk.Muted.Render(shared.PadOrTruncateVisible("(select a row)", width))
		return empty
	}
	half := height / 2
	if half < 4 {
		half = 4
	}
	entry := m.cursorEntry()
	reqPlain := buildRequestLines(entry)
	respPlain := buildResponseLines(entry)
	reqStyled := styledRequestLines(entry, tk)
	respStyled := styledResponseLines(entry, tk)

	reqBox := m.renderPaneBox("REQUEST", APIRecorderPaneRequest,
		reqPlain, reqStyled, m.requestCursor, width, half, tk)
	respBox := m.renderPaneBox("RESPONSE", APIRecorderPaneResponse,
		respPlain, respStyled, m.responseCursor, width, height-half, tk)
	return reqBox + "\n" + respBox
}

// renderPaneBox draws one labeled section with cursor + visual
// styling applied to the focused pane's lines. plainLines and
// styledLines must have identical length / index alignment — the
// cursor + visual selection address the plain slice (so yank carries
// pristine text) while the visible render reads from the styled
// slice (so syntax tints survive).
func (m APIRecorderModel) renderPaneBox(label string, paneID APIRecorderPane, plainLines, styledLines []string, cur shared.BodyCursor, width, height int, tk shared.Tokens) string {
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
	total := len(plainLines)
	if len(styledLines) < total {
		// Defensive: pad styledLines with the plain fallback so the
		// loop below never indexes past the slice on a build error.
		extra := make([]string, total-len(styledLines))
		for i := range extra {
			extra[i] = plainLines[len(styledLines)+i]
		}
		styledLines = append(styledLines, extra...)
	}
	cur.EnsureVisible(bodyHeight, total)
	visualStart, visualEnd := cur.VisualRange()
	from := cur.Top
	if from < 0 {
		from = 0
	}
	if from > total {
		from = total
	}
	to := from + bodyHeight
	if to > total {
		to = total
	}

	out := []string{header}
	for i := 0; i < bodyHeight; i++ {
		idx := from + i
		if idx >= total {
			out = append(out, strings.Repeat(" ", width))
			continue
		}
		styled := styledLines[idx]
		prefix := "  "
		if m.pane == paneID && idx == cur.Line {
			prefix = "▸ "
		}
		// The cursor / visual highlights strip styling and re-render
		// with RowCursor so the bg highlight reads cleanly across the
		// whole row — losing the per-token syntax tints is the
		// trade-off, mirroring how every other detail surface does it.
		switch {
		case m.pane == paneID && idx == cur.Line:
			full := shared.PadOrTruncateVisible(prefix+plainLines[idx], width)
			out = append(out, tk.RowCursor.Render(shared.StripCSI(full)))
		case m.pane == paneID && cur.Visual && idx >= visualStart && idx <= visualEnd:
			full := shared.PadOrTruncateVisible(prefix+plainLines[idx], width)
			out = append(out, tk.RowCursor.Render(shared.StripCSI(full)))
		default:
			// Styled rows preserve their ANSI sequences — pad with
			// trailing spaces only, never truncate (truncation would
			// chop mid-CSI and leak escape codes downstream).
			line := prefix + styled
			if shared.VisibleWidth(line) > width {
				// Overflow — fall back to a stripped truncate so we
				// don't blow the pane width. Rare in practice (most
				// JSON lines fit easily in 60-cell pane).
				stripped := shared.StripCSI(line)
				line = shared.PadOrTruncateVisible(stripped, width)
			} else {
				line = padTrailing(line, width)
			}
			out = append(out, line)
		}
	}
	return strings.Join(out, "\n")
}

// formatTimelineRow returns the un-styled timeline row used by the
// BodyCursor for yank operations (clipboard text shouldn't carry
// ANSI codes) and for the cursor-row highlight render path (which
// strips styling and re-renders with RowCursor). The visible View
// uses renderTimelineRowStyled instead — that path applies per-cell
// tints and the status badge.
func formatTimelineRow(e apilog.Entry, pathWidth int) string {
	g := strings.Repeat(" ", tlGutter)
	stamp := padRightAR(e.Time.Local().Format("15:04:05"), tlColTime)
	method := padRightAR(e.Method, tlColMethod)
	path := shared.PadOrTruncateVisible(e.Path, pathWidth)
	var status string
	switch {
	case e.Err != "":
		status = "ERR"
	case e.Status > 0:
		status = strconv.Itoa(e.Status)
	default:
		status = "---"
	}
	statusCell := centerAR(status, tlColStatus)
	dur := padLeftAR(formatDurationMS(e.DurationMS), tlColDuration)
	return stamp + g + method + g + path + g + statusCell + g + dur
}

// buildRequestLines flattens the request fields into renderable
// lines. The plain-text variant (used for yank + cursor accounting)
// returns lines without any ANSI styling so clipboard pastes don't
// carry escape sequences. The styled variant lives in
// styledRequestLines.
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
		formatDurationMS(e.DurationMS)
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

// styledRequestLines returns the request body with syntax styling
// applied to each line. The line count and ordering match
// buildRequestLines exactly so the BodyCursor's line index addresses
// the same content in both views.
func styledRequestLines(e apilog.Entry, tk shared.Tokens) []string {
	if e.SeqID == 0 && e.Method == "" {
		return []string{tk.Muted.Render("(no entry selected)")}
	}
	var out []string
	out = append(out, methodStyle(e.Method, tk).Bold(true).Render(e.Method)+
		"  "+tk.FG.Render(e.URL))
	out = append(out, "")
	out = append(out, styledHeaderLines(e.RequestHeaders, tk)...)
	if e.RequestBody != "" {
		out = append(out, "")
		out = append(out, tk.Muted.Render("Body:"))
		out = append(out, styledBodyLines(e.RequestBody, tk)...)
	}
	return out
}

// styledResponseLines mirrors styledRequestLines for the response
// pane. The status line carries the per-class background tint so
// the response pane shows the same color cue as the timeline.
func styledResponseLines(e apilog.Entry, tk shared.Tokens) []string {
	if e.SeqID == 0 && e.Method == "" {
		return []string{tk.Muted.Render("(no entry selected)")}
	}
	statusBadgeText := statusBadgeForBody(e, tk)
	dur := tk.Muted.Render(formatDurationMS(e.DurationMS))
	statusLine := statusBadgeText + "  " + dur
	if e.Err != "" {
		statusLine += "  " + tk.Danger.Render(e.Err)
	}
	out := []string{statusLine, ""}
	out = append(out, styledHeaderLines(e.ResponseHeaders, tk)...)
	if e.ResponseBody != "" {
		out = append(out, "")
		out = append(out, tk.Muted.Render("Body:"))
		out = append(out, styledBodyLines(e.ResponseBody, tk)...)
	}
	return out
}

// statusBadgeForBody renders the inline status badge used at the top
// of the Response pane. Same color map as the timeline column but
// without the fixed 5-cell width — the body pane has more room.
func statusBadgeForBody(e apilog.Entry, tk shared.Tokens) string {
	if e.Err != "" {
		return tk.BadgeUnmask.Render(" ERR ")
	}
	if e.Status <= 0 {
		return tk.Muted.Render(" --- ")
	}
	cell := " " + strconv.Itoa(e.Status) + " "
	switch {
	case e.Status >= 500:
		return tk.BadgeUnmask.Render(cell)
	case e.Status >= 400:
		return tk.BadgeLarge.Render(cell)
	case e.Status >= 200 && e.Status < 300:
		return tk.BadgeRule.Render(cell)
	default:
		return tk.Accent.Render(cell)
	}
}

// styledHeaderLines tints the header key (Accent) and value (FG) so
// long header dumps stay scannable. Sensitive values arrive already
// scrubbed (`***`) from apilog.RedactHeaders, which the Muted style
// further de-emphasizes.
func styledHeaderLines(h http.Header, tk shared.Tokens) []string {
	if len(h) == 0 {
		return []string{tk.Muted.Render("(no headers)")}
	}
	keys := make([]string, 0, len(h))
	for k := range h {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := make([]string, 0, len(h))
	for _, k := range keys {
		for _, v := range h[k] {
			valueStyle := tk.FG
			if v == apilog.RedactedToken {
				valueStyle = tk.Muted
			}
			line := tk.Accent.Render(k) + tk.Muted.Render(": ") + valueStyle.Render(v)
			out = append(out, line)
		}
	}
	return out
}

// styledBodyLines runs the body through HighlightJSON when it parses
// as JSON; otherwise falls back to a plain split so non-JSON content
// (e.g., a 502 HTML page) still renders without crashing the
// highlighter. Trailing "[truncated]" markers from CapBody stay
// muted.
func styledBodyLines(body string, tk shared.Tokens) []string {
	trimmed := strings.TrimSpace(body)
	if looksLikeJSON(trimmed) {
		highlighted := shared.HighlightJSON(body, tk)
		return splitLines(highlighted)
	}
	return splitLines(body)
}

// looksLikeJSON is a fast prefix sniff so we don't pay an unmarshal
// cost on every render. Bodies that start with `{` / `[` after
// trimming are treated as JSON; everything else (HTML, plain text,
// XML) falls through to the un-highlighted path.
func looksLikeJSON(s string) bool {
	if len(s) == 0 {
		return false
	}
	c := s[0]
	return c == '{' || c == '['
}

// headerLines returns "Key: value" lines sorted by key — used by the
// plain-text path for yank operations.
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

// padLeftAR right-aligns s to n cells (pad with leading spaces).
func padLeftAR(s string, n int) string {
	w := shared.VisibleWidth(s)
	if w >= n {
		return s
	}
	return strings.Repeat(" ", n-w) + s
}

// centerAR pads s to n cells with leading + trailing spaces so the
// content sits centered within the cell. Used for the status badge.
func centerAR(s string, n int) string {
	w := shared.VisibleWidth(s)
	if w >= n {
		return s
	}
	pad := n - w
	left := pad / 2
	right := pad - left
	return strings.Repeat(" ", left) + s + strings.Repeat(" ", right)
}

// padTrailing right-pads an already-styled string to n visible
// cells. Unlike PadOrTruncateVisible this never truncates — it's
// for the status-badge row whose styled cells already match the
// canonical width and just need trailing spaces to reach the pane
// boundary without clipping the embedded ANSI codes.
func padTrailing(s string, n int) string {
	w := shared.VisibleWidth(s)
	if w >= n {
		return s
	}
	return s + strings.Repeat(" ", n-w)
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
