package app

import "testing"

func TestCommandLineChoosesSourceOverEnvironment(t *testing.T) {
	t.Setenv("JSON_FILE", "examples/laureates.json")
	a, err := New([]string{"-url", "https://example.test/laureates.json"})
	if err != nil {
		t.Fatal(err)
	}
	if a.cfg.JSONFile != "" || a.cfg.SourceURL != "https://example.test/laureates.json" {
		t.Fatalf("URL flag did not select HTTP source: %+v", a.cfg)
	}
	a, err = New([]string{"-file", "other.json"})
	if err != nil || a.cfg.JSONFile != "other.json" {
		t.Fatalf("file flag did not override environment: %v", err)
	}
	if _, err := New([]string{"-file", "other.json", "-url", "https://example.test"}); err == nil {
		t.Fatal("expected conflicting source flags to fail")
	}
}
