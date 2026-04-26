package config

// PickProfile selects an active profile based on, in order:
// 1. --profile flag (explicit)
// 2. a single profile in the file (auto-select)
// 3. prompts the caller (returns ErrNoProfile; UI shows Profile Select SCR-000)
func PickProfile(cfg Config, flagProfile string) (name string, p Profile, err error) {
	panic("config.PickProfile: not implemented yet")
}

// ResolveToken returns the SSWS API token and a short human-readable source
// tag ("env OKTA_API_TOKEN", "interactive prompt") for `:about` (REQ-C04).
// Does not persist the token anywhere (REQ-C05).
func ResolveToken(flagTokenEnv string, p Profile, interactive bool) (token, source string, err error) {
	panic("config.ResolveToken: not implemented yet")
}
