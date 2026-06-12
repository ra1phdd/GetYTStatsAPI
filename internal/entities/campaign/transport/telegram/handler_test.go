package campaign_telegram

import "testing"

func TestParseHumanDateSupportsExtendedFormats(t *testing.T) {
	t.Parallel()

	inputs := []string{
		"12-06-26",
		"12.06.2026",
		"12 06 2026",
		"12 06 26",
		"12-06-2026",
	}

	for _, input := range inputs {
		parsed, err := parseHumanDate(input)
		if err != nil {
			t.Fatalf("parseHumanDate(%q) error = %v", input, err)
		}
		if got := parsed.Format("2006-01-02"); got != "2026-06-12" {
			t.Fatalf("parseHumanDate(%q) = %s, want 2026-06-12", input, got)
		}
	}
}
