package service

// Bundle groups all services for easy wiring (cmd/ota/wire.go -> app.Model).
type Bundle struct {
	Users    *UsersService
	Groups   *GroupsService
	Rules    *GroupRulesService
	Policies *PoliciesService
	Logs     *LogsService
	// LogsTail drives the stateful tail loop (REQ-R05 AC-2/AC-3). Separate
	// from LogsService because it owns since-cursor + adaptive poll state.
	LogsTail *LogsTail
}

// InvalidateAll clears every service cache (called on `:profile` switch).
func (b *Bundle) InvalidateAll() {
	if b.Users != nil {
		b.Users.Invalidate()
	}
	if b.Groups != nil {
		b.Groups.Invalidate()
	}
	if b.Rules != nil {
		b.Rules.Invalidate()
	}
	if b.Policies != nil {
		b.Policies.Invalidate()
	}
}
