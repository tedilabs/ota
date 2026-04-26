package keys

import "fmt"

// ResolvedMap is the final ID → keybinding string mapping applied by the app.
type ResolvedMap map[ID]string

// Resolve merges Defaults() with user overrides. Unknown user IDs produce a
// warning (REQ-C03 AC-3) but never an error — boot continues with the
// built-in mapping for that slot.
//
// The returned warnings slice is nil when there are no issues.
func Resolve(userOverrides map[string]string) (ResolvedMap, []string, error) {
	defaults := Defaults()
	known := allIDs()

	resolved := make(ResolvedMap, len(defaults))
	for id, kb := range defaults {
		resolved[id] = kb
	}

	var warnings []string
	for rawID, kb := range userOverrides {
		id := ID(rawID)
		if _, ok := known[id]; !ok {
			warnings = append(warnings, fmt.Sprintf("unknown key id %q — ignored", rawID))
			continue
		}
		resolved[id] = kb
	}
	return resolved, warnings, nil
}

// Reverse returns the inverse lookup: keybinding string → ID. Used by the
// Update handlers to classify tea.KeyMsg events.
//
// When multiple IDs share the same keybinding (possible after overrides),
// the later iteration wins; callers should check warnings from Resolve for
// conflict hints.
func (m ResolvedMap) Reverse() map[string]ID {
	out := make(map[string]ID, len(m))
	for id, kb := range m {
		out[kb] = id
	}
	return out
}
