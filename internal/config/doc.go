// Package config loads and validates ota's YAML configuration with XDG path
// resolution and layered merge (default → file → env) via knadh/koanf.
//
// See docs/CONVENTIONS.md §7 for key schema and REQ-C01~C05 for requirements.
package config
