package shared

import (
	"regexp"
	"strings"
)

// HighlightJSON returns s with each JSON token wrapped in the appropriate
// lipgloss style. Operates one line at a time so output structure (indent,
// braces) is preserved verbatim. Lines containing the v0.1.1 mask token
// "***" still flow through annotateMaskedLines's "# masked" comment without
// being mistaken for a string value.
//
// The colour map (issue #110):
//
//	"key":             tk.Accent  (cyan in dark theme)
//	"string value"     tk.Success (green)
//	123 / 4.5          tk.Magenta (number)
//	true / false       tk.Warning (bool)
//	null               tk.Muted   (null)
//	# masked comment   tk.Muted   (comment)
//
// Brackets, commas, and whitespace stay untinted so the structure remains
// easy to scan.
func HighlightJSON(s string, tk Tokens) string {
	if MonochromeEnabled() {
		return s
	}
	out := make([]string, 0, strings.Count(s, "\n")+1)
	for _, line := range strings.Split(s, "\n") {
		out = append(out, highlightJSONLine(line, tk))
	}
	return strings.Join(out, "\n")
}

// HighlightYAML returns s with each YAML token wrapped in the appropriate
// lipgloss style. Same colour map as HighlightJSON, adapted for YAML's
// `key: value` form.
func HighlightYAML(s string, tk Tokens) string {
	if MonochromeEnabled() {
		return s
	}
	out := make([]string, 0, strings.Count(s, "\n")+1)
	for _, line := range strings.Split(s, "\n") {
		out = append(out, highlightYAMLLine(line, tk))
	}
	return strings.Join(out, "\n")
}

// jsonKeyRE matches a JSON key: leading whitespace, "key", colon. Group 1
// captures the indent, group 2 the quoted key.
var jsonKeyRE = regexp.MustCompile(`^(\s*)("(?:[^"\\]|\\.)*")\s*:`)

// jsonNumberRE matches a JSON number literal token after a colon.
var jsonNumberRE = regexp.MustCompile(`(:\s*)(-?\d+(?:\.\d+)?(?:[eE][+-]?\d+)?)([,\s\]}]|$)`)

// jsonBoolNullRE matches true / false / null after a colon.
var jsonBoolNullRE = regexp.MustCompile(`(:\s*)(true|false|null)([,\s\]}]|$)`)

// jsonStringValRE matches a quoted string value after a colon.
var jsonStringValRE = regexp.MustCompile(`(:\s*)("(?:[^"\\]|\\.)*")`)

// commentRE strips off any trailing " # masked" annotation so highlighting
// doesn't try to interpret the mask token as JSON/YAML content.
var commentRE = regexp.MustCompile(`\s*#\s.*$`)

func highlightJSONLine(line string, tk Tokens) string {
	body, comment := splitTrailingComment(line)
	if m := jsonKeyRE.FindStringSubmatchIndex(body); m != nil {
		indent := body[m[2]:m[3]]
		key := body[m[4]:m[5]]
		rest := body[m[1]:]
		body = indent + tk.Accent.Render(key) + ":" + colorJSONValue(rest, tk)
	} else {
		// No key on this line (e.g. an array element or trailing brace).
		body = colorJSONValue(":"+body, tk)[1:]
	}
	if comment != "" {
		body = body + tk.Muted.Render(comment)
	}
	return body
}

// colorJSONValue applies value-side colour to the slice between the colon
// and end-of-line. Expects to receive everything after the key including
// the leading colon-and-spaces so the regexes match anchor patterns.
func colorJSONValue(rest string, tk Tokens) string {
	rest = jsonStringValRE.ReplaceAllStringFunc(rest, func(m string) string {
		sm := jsonStringValRE.FindStringSubmatch(m)
		return sm[1] + tk.Success.Render(sm[2])
	})
	rest = jsonBoolNullRE.ReplaceAllStringFunc(rest, func(m string) string {
		sm := jsonBoolNullRE.FindStringSubmatch(m)
		style := tk.Warning
		if sm[2] == "null" {
			style = tk.Muted
		}
		return sm[1] + style.Render(sm[2]) + sm[3]
	})
	rest = jsonNumberRE.ReplaceAllStringFunc(rest, func(m string) string {
		sm := jsonNumberRE.FindStringSubmatch(m)
		return sm[1] + tk.Magenta.Render(sm[2]) + sm[3]
	})
	return rest
}

// yamlKeyRE matches a YAML key: leading indent, key (unquoted up to colon),
// colon. Captures (1) indent, (2) key text, (3) the rest of the line after
// `: `.
var yamlKeyRE = regexp.MustCompile(`^(\s*-?\s*)([A-Za-z0-9_./-]+)\s*:(\s*.*)$`)

// yamlBoolNullRE / yamlNumberRE / yamlStringRE match scalar values to the
// right of a YAML key. The string form deliberately covers both quoted and
// bare values; quotes are coloured with the value.
var (
	yamlBoolNullRE = regexp.MustCompile(`^(true|false|null|~)$`)
	yamlNumberRE   = regexp.MustCompile(`^(-?\d+(?:\.\d+)?(?:[eE][+-]?\d+)?)$`)
)

func highlightYAMLLine(line string, tk Tokens) string {
	body, comment := splitTrailingComment(line)
	m := yamlKeyRE.FindStringSubmatch(body)
	if m == nil {
		return body + maybeRender(comment, tk.Muted)
	}
	indent, key, rest := m[1], m[2], strings.TrimRight(m[3], " ")
	if rest == "" || strings.HasPrefix(rest, " {}") || strings.HasPrefix(rest, " []") {
		// key with no scalar value (parent of a nested block / empty map).
		return indent + tk.Accent.Render(key) + ":" + rest + maybeRender(comment, tk.Muted)
	}
	value := strings.TrimLeft(rest, " ")
	leading := rest[:len(rest)-len(value)]
	value = strings.TrimRight(value, " ")
	switch {
	case yamlBoolNullRE.MatchString(value):
		style := tk.Warning
		if value == "null" || value == "~" {
			style = tk.Muted
		}
		value = style.Render(value)
	case yamlNumberRE.MatchString(value):
		value = tk.Magenta.Render(value)
	default:
		value = tk.Success.Render(value)
	}
	return indent + tk.Accent.Render(key) + ":" + leading + value + maybeRender(comment, tk.Muted)
}

// splitTrailingComment slices off a trailing "# …" comment (with leading
// whitespace) so the highlighter doesn't reinterpret it. Returns the body
// and the comment (with its preceding whitespace) as separate strings.
func splitTrailingComment(s string) (string, string) {
	loc := commentRE.FindStringIndex(s)
	if loc == nil {
		return s, ""
	}
	return s[:loc[0]], s[loc[0]:]
}

func maybeRender(s string, style interface{ Render(...string) string }) string {
	if s == "" {
		return ""
	}
	return style.Render(s)
}
