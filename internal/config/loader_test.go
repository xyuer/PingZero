package config

import "testing"

func TestLoadAll(t *testing.T) {
	bundle, err := LoadAll("../../config")
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := bundle.Games["lol"]; !ok {
		t.Fatal("expected lol config")
	}
}

func TestLoadAllMissingDir(t *testing.T) {
	if _, err := LoadAll("missing"); err == nil {
		t.Fatal("expected missing config error")
	}
}
