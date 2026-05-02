package tui

func BoldText(s string) string {
	if s == "" || !ColorsEnabled {
		return s
	}
	return Bold + s + ResetIntensity
}

func DimText(s string) string {
	if s == "" || !ColorsEnabled {
		return s
	}
	return Muted + s + ResetFG
}

func HighlightText(s string) string {
	if s == "" || !ColorsEnabled {
		return s
	}
	return Highlight + s + ResetFG + ResetIntensity
}

func AccentText(s string) string {
	if s == "" || !ColorsEnabled {
		return s
	}
	return Accent + s + ResetFG + ResetIntensity
}
