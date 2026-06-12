package youtube_repository

import "testing"

func TestParseChannelReference(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		kind  channelReferenceKind
		value string
	}{
		{name: "raw channel id", input: "UCoV1F9JNNqripLok4BDLetQ", kind: channelReferenceID, value: "UCoV1F9JNNqripLok4BDLetQ"},
		{name: "channel url", input: "https://www.youtube.com/channel/UCoV1F9JNNqripLok4BDLetQ/", kind: channelReferenceID, value: "UCoV1F9JNNqripLok4BDLetQ"},
		{name: "studio channel url", input: "https://studio.youtube.com/channel/UCoV1F9JNNqripLok4BDLetQ/videos", kind: channelReferenceID, value: "UCoV1F9JNNqripLok4BDLetQ"},
		{name: "handle url", input: "https://www.youtube.com/@drake_afk", kind: channelReferenceHandle, value: "drake_afk"},
		{name: "raw handle", input: "@drake_afk", kind: channelReferenceHandle, value: "drake_afk"},
		{name: "username url", input: "https://www.youtube.com/user/google", kind: channelReferenceUsername, value: "google"},
		{name: "youtube without scheme", input: "www.youtube.com/@drake_afk", kind: channelReferenceHandle, value: "drake_afk"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ref, err := parseChannelReference(tt.input)
			if err != nil {
				t.Fatalf("parseChannelReference(%q) error = %v", tt.input, err)
			}
			if ref.kind != tt.kind || ref.value != tt.value {
				t.Fatalf("parseChannelReference(%q) = {%q %q}, want {%q %q}", tt.input, ref.kind, ref.value, tt.kind, tt.value)
			}
		})
	}
}

func TestParseChannelReferenceRejectsUnsupportedURL(t *testing.T) {
	t.Parallel()

	if _, err := parseChannelReference("https://example.com/channel/test"); err == nil {
		t.Fatal("parseChannelReference() error = nil, want error")
	}
}
