package config

import "testing"

func TestDefaultTtsProvider(t *testing.T) {
	if Conf.Tts.Provider != "edge-tts" {
		t.Fatalf("Tts.Provider = %q, want edge-tts", Conf.Tts.Provider)
	}
}

func TestDefaultTranscribeProvider(t *testing.T) {
	if Conf.Transcribe.Provider != "openai" {
		t.Fatalf("Transcribe.Provider = %q, want openai", Conf.Transcribe.Provider)
	}
}
