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

func TestParseNotificationIntervalText(t *testing.T) {
	t.Parallel()

	tests := map[string]int{
		"3ч": 180,
		"6h": 360,
		"7д": 10080,
		"3d": 4320,
	}

	for input, want := range tests {
		got, err := parseNotificationIntervalText(input)
		if err != nil {
			t.Fatalf("parseNotificationIntervalText(%q) error = %v", input, err)
		}
		if got != want {
			t.Fatalf("parseNotificationIntervalText(%q) = %d, want %d", input, got, want)
		}
	}
}
