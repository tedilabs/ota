package apps

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/tedilabs/ota/internal/domain"
)

// Wrapper owns the app-type-select → list flow (issue #166), mirroring
// the Policies wrapper. Mounted as ScreenApps in the App Shell.
type Wrapper struct {
	deps     Deps
	mode     wrapperMode
	selector TypeSelectModel
	list     ListModel
	picked   domain.AppType
}

type wrapperMode int

const (
	wrapperModeSelect wrapperMode = iota
	wrapperModeList
)

func NewWrapper(deps Deps) Wrapper {
	return Wrapper{
		deps:     deps,
		mode:     wrapperModeSelect,
		selector: NewTypeSelectModel(deps),
	}
}

// NewWrapperForType opens directly on the typed list — used by the
// per-type palette routes (`:saml-app`, `:oidc-app`, …).
func NewWrapperForType(deps Deps, t domain.AppType) Wrapper {
	return Wrapper{
		deps:   deps,
		mode:   wrapperModeList,
		picked: t,
		list:   NewListModel(deps, t),
	}
}

func (w Wrapper) Init() tea.Cmd {
	if w.mode == wrapperModeList {
		return w.list.Init()
	}
	return w.selector.Init()
}

func (w Wrapper) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Cross-screen drill-down (issue #171): handled at Wrapper level so
	// it works regardless of the current mode (the operator may not have
	// picked a type yet).
	switch m := msg.(type) {
	case OpenDetailByIDMsg:
		if m.ID == "" || w.deps.Port == nil {
			return w, nil
		}
		return w, fetchAppByIDCmd(w.deps.Port, m.ID)
	case appOpenedByIDMsg:
		t := m.app.Type
		if t == "" {
			t = domain.AppTypeOther
		}
		w.picked = t
		w.list = NewListModel(w.deps, t)
		w.list.apps = []domain.App{m.app}
		w.list.detail = m.app
		w.list.opened = true
		w.list.detailTab = AppDetailTabPretty
		w.mode = wrapperModeList
		w.selector = NewTypeSelectModel(w.deps)
		return w, nil
	case appOpenByIDErrMsg:
		// Surface the failure on the list's error panel so the operator
		// sees something rather than a silent no-op. Keep the wrapper
		// mode as-is.
		if w.mode == wrapperModeList {
			w.list.lastErr = m.err
		}
		return w, nil
	}
	switch w.mode {
	case wrapperModeSelect:
		updated, cmd := w.selector.Update(msg)
		w.selector = updated.(TypeSelectModel)
		if t, ok := w.selector.Picked(); ok {
			w.picked = t
			w.list = NewListModel(w.deps, t)
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
		if km, ok := msg.(tea.KeyMsg); ok && km.Type == tea.KeyEsc {
			if !w.list.opened && w.list.filter == "" && !w.list.filtering {
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

func (w Wrapper) View() string {
	switch w.mode {
	case wrapperModeList:
		return w.list.View()
	default:
		return w.selector.View()
	}
}

// Filtering / Filter / Count proxy through to the active list so the
// App Shell's chrome integrations work in either mode.
func (w Wrapper) Filtering() bool {
	if w.mode == wrapperModeList {
		return w.list.Filtering()
	}
	return false
}
func (w Wrapper) Filter() string {
	if w.mode == wrapperModeList {
		return w.list.Filter()
	}
	return ""
}
func (w Wrapper) Count() (visible, total int) {
	if w.mode == wrapperModeList {
		return w.list.Count()
	}
	return 0, 0
}

// AppType reports the currently-selected type (zero value while the
// selector is active). Exposed for tests.
func (w Wrapper) AppType() domain.AppType { return w.picked }

// Mode reports the active inner Model — "select" or "list". Exposed
// for tests.
func (w Wrapper) Mode() string {
	if w.mode == wrapperModeList {
		return "list"
	}
	return "select"
}

var _ tea.Model = Wrapper{}
