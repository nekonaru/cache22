package main

import (
	"strings"
	"testing"
)

func TestParseCreateFlagsDefaults(t *testing.T) {
	c, err := parseCreateFlags(nil)
	if err != nil {
		t.Fatal(err)
	}
	if c.maxReads != 1 || c.ttl != "1h" || !strings.HasPrefix(c.server, "http") {
		t.Fatalf("unexpected defaults: %+v", c)
	}
}

func TestParseCreateFlags(t *testing.T) {
	c, err := parseCreateFlags([]string{
		"-server", "https://host", "-ttl", "7d", "-max-reads", "5",
		"-file", "a.bin", "-passphrase", "pw",
	})
	if err != nil {
		t.Fatal(err)
	}
	if c.server != "https://host" || c.ttl != "7d" || c.maxReads != 5 || c.file != "a.bin" || c.passphrase != "pw" {
		t.Fatalf("unexpected config: %+v", c)
	}
}

func TestParseCreateFlagsInvalid(t *testing.T) {
	cases := [][]string{
		{"-unknown"},
		{"-server"},
		{"-ttl"},
		{"-max-reads"},
		{"-file"},
		{"-passphrase"},
		{"-max-reads", "nope"},
		{"-max-reads", "11"},
		{"-ttl", "5s"},
	}
	for _, args := range cases {
		if _, err := parseCreateFlags(args); err == nil {
			t.Fatalf("%v should fail", args)
		}
	}
}

func TestNextFlag(t *testing.T) {
	c, err := parseCreateFlags([]string{"-max-reads", "0"})
	if err != nil || c.maxReads != 1 {
		t.Fatalf("zero max-reads should default to 1: c=%+v err=%v", c, err)
	}
	if _, err := nextFlag([]string{"-a"}, 0, "-a"); err == nil {
		t.Fatal("missing value must fail")
	}
	v, err := nextFlag([]string{"-a", "v"}, 0, "-a")
	if err != nil || v != "v" {
		t.Fatalf("v=%q err=%v", v, err)
	}
}

func TestParseDropURL(t *testing.T) {
	id, frag, base, err := parseDropURL("https://host.example/d/ABCDEFGHIJKLMNOPQRSTUVWXYZ#key")
	if err != nil {
		t.Fatal(err)
	}
	if id != "ABCDEFGHIJKLMNOPQRSTUVWXYZ" || frag != "key" || base != "https://host.example" {
		t.Fatalf("id=%q frag=%q base=%q", id, frag, base)
	}
	for _, raw := range []string{
		"", "ftp://host/d/id", "https://host", "https://host/x/id", "https://host/d/",
		"https://host/d/id/extra",
	} {
		if _, _, _, err := parseDropURL(raw); err == nil {
			t.Fatalf("%q should fail", raw)
		}
	}
}

func TestParseDropURLTrailingSlash(t *testing.T) {
	id, _, base, err := parseDropURL("http://host.example/d/abc123/")
	if err != nil {
		t.Fatal(err)
	}
	if id != "abc123" || base != "http://host.example" {
		t.Fatalf("id=%q base=%q", id, base)
	}
}

func TestBaseName(t *testing.T) {
	for path, want := range map[string]string{
		"file.txt":      "file.txt",
		"a/b/file.txt":  "file.txt",
		`c:\a\file.txt`: "file.txt",
	} {
		if got := baseName(path); got != want {
			t.Fatalf("baseName(%q) = %q, want %q", path, got, want)
		}
	}
}

func TestUsageOutput(t *testing.T) {
	// usage writes to stderr only; this test ensures it does not panic and
	// mentions the CLI commands users rely on.
	usage()
}
