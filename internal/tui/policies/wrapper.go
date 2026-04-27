package policies

// Wrapper owns the policy-type-select → list flow (issue #154).
// The App Shell mounts a single Wrapper as its ScreenPolicies; the
// wrapper renders a TypeSelectModel until the operator picks a
// type, then swaps to a ListModel for that type. Esc on the list
// returns to the type select so a new type can be picked without
// leaving the screen.

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/tedilabs/ota/internal/domain"
)

// Wrapper is the public Policies screen — a Bubbletea Model that
// switches between TypeSelectModel and ListModel internally.
type Wrapper struct {
	deps     Deps
	mode     wrapperMode
	selector TypeSelectModel
	list     ListModel
	picked   domain.PolicyType
}

type wrapperMode int

const (
	wrapperModeSelect wrapperMode = iota
	wrapperModeList
)

// NewWrapper constructs a Wrapper starting on the type-select screen.
func NewWrapper(deps Deps) Wrapper {
	return Wrapper{
		deps:     deps,
		mode:     wrapperModeSelect,
		selector: NewTypeSelectModel(deps),
	}
}

// Init implements tea.Model.
func (w Wrapper) Init() tea.Cmd { return w.selector.Init() }

// Update routes incoming messages to the active inner model and
// handles the TypeSelect → List transition when Enter selects a type.
func (w Wrapper) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch w.mode {
	case wrapperModeSelect:
		updated, cmd := w.selector.Update(msg)
		w.selector = updated.(TypeSelectModel)
		if t, ok := w.selector.Picked(); ok {
			// Enter pressed → swap to a ListModel for the picked
			// type. Reset the selector so a return-trip starts
			// from a clean state.
			w.picked = t
			w.list = NewListModel(w.deps, t)
			// Pipe through any cmd from the selector (none today,
			// but future-proof) before the list's Init Cmd.
			w.mode = wrapperModeList
			w.selector = NewTypeSelectModel(w.deps)
			if init := w.list.Init(); init != nil {
				if cmd != nil {
					return w, tea.Batch(cmd, init)
				}
				return w, init
			}
			return w, cmd
		}
		return w, cmd
	case wrapperModeList:
		// Esc on the list (with no detail open and no filter set)
		// pops back to the type-select screen so the operator can
		// pick a different type without leaving the chrome.
		if km, ok := msg.(tea.KeyMsg); ok && km.Type == tea.KeyEsc {
			if !w.list.opened {
				w.mode = wrapperModeSelect
				w.list = ListModel{}
				return w, nil
			}
		}
		updated, cmd := w.list.Update(msg)
		w.list = updated.(ListModel)
		return w, cmd
	}
	return w, nil
}

// View implements tea.Model.
func (w Wrapper) View() string {
	switch w.mode {
	case wrapperModeList:
		return w.list.View()
	default:
		return w.selector.View()
	}
}

// Count surfaces the active list's count to the App Shell so the
// chrome's upper divider can stamp "N of M" — only meaningful in
// list mode.
func (w Wrapper) Count() (visible, total int) {
	if w.mode == wrapperModeList {
		return w.list.Count()
	}
	return 0, 0
}

// PolicyType reports the currently-selected type (zero value while
// the selector is active). Exposed for tests.
func (w Wrapper) PolicyType() domain.PolicyType { return w.picked }

// Mode reports whether the wrapper is currently showing the type
// selector or the list. Exposed for tests.
func (w Wrapper) Mode() string {
	if w.mode == wrapperModeList {
		return "list"
	}
	return "select"
}

var _ tea.Model = Wrapper{}
