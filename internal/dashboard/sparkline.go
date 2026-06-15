package dashboard

// sparklineGlyphs is the 9-step block ramp every TUI sparkline in
// ota uses. Index 0 → space (no data / zero), 8 → full block. The
// ramp is the same one k9s / lazygit / htop converged on.
var sparklineGlyphs = []string{" ", "▁", "▂", "▃", "▄", "▅", "▆", "▇", "█"}

// RenderSparkline stamps a row of block glyphs from a slice of
// pre-normalized 0..8 bucket indices. The home screen passes the
// output of Delta.Sparkline straight in.
func RenderSparkline(buckets []int) string {
	if len(buckets) == 0 {
		return ""
	}
	out := make([]byte, 0, len(buckets)*3) // multi-byte UTF-8 per glyph
	for _, b := range buckets {
		if b < 0 {
			b = 0
		}
		if b >= len(sparklineGlyphs) {
			b = len(sparklineGlyphs) - 1
		}
		out = append(out, sparklineGlyphs[b]...)
	}
	return string(out)
}

// NormalizeSparkline converts a slice of raw values into the 0..8
// bucket form RenderSparkline consumes. Exported because Phase 4's
// Activity card builds its own series (hourly sign-in counts)
// outside the daily-history path DeltaFor walks.
func NormalizeSparkline(values []int) []int {
	if len(values) == 0 {
		return nil
	}
	min, max := values[0], values[0]
	for _, v := range values {
		if v < min {
			min = v
		}
		if v > max {
			max = v
		}
	}
	out := make([]int, len(values))
	if max == min {
		for i := range out {
			out[i] = 4
		}
		return out
	}
	span := float64(max - min)
	for i, v := range values {
		out[i] = int(float64(v-min) / span * 8.0)
		if out[i] > 8 {
			out[i] = 8
		}
		if out[i] < 0 {
			out[i] = 0
		}
	}
	return out
}
