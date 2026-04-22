package main

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"
)

func TestArchiveBaseName(t *testing.T) {
	cases := map[string]string{
		"archive.tar.gz": "archive",
		"archive.zip":    "archive",
		"archive":        "archive",
		"/tmp/a.bin":     "a",
		"-":              "archive",
	}
	for input, want := range cases {
		if got := archiveBaseName(input); got != want {
			t.Fatalf("archiveBaseName(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestPrintUsageHasNoMissingPlaceholders(t *testing.T) {
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()
	os.Args = []string{"arklite"}

	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w
	defer func() { os.Stdout = oldStdout }()

	printUsage()
	_ = w.Close()
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if strings.Contains(out, "%!s(MISSING)") {
		t.Fatalf("usage output contains missing placeholder: %q", out)
	}
	if !strings.Contains(out, "install") {
		t.Fatalf("usage output missing install command: %q", out)
	}
}

func TestDetectSharedRoot(t *testing.T) {
	cases := []struct {
		name  string
		paths []string
		want  string
	}{
		{name: "shared root", paths: []string{"example/", "example/program", "example/docs/readme.md"}, want: "example"},
		{name: "mixed roots", paths: []string{"example/program", "other/file.txt"}, want: ""},
		{name: "top level file", paths: []string{"example/program", "file.txt"}, want: ""},
		{name: "single root dir only", paths: []string{"example/"}, want: ""},
	}
	for _, tc := range cases {
		if got := detectSharedRoot(tc.paths); got != tc.want {
			t.Fatalf("%s: detectSharedRoot(%q) = %q, want %q", tc.name, tc.paths, got, tc.want)
		}
	}
}

func TestStripSharedRoot(t *testing.T) {
	cases := []struct {
		name string
		path string
		root string
		want string
	}{
		{name: "strip nested file", path: "example/program", root: "example", want: "program"},
		{name: "strip nested dir", path: "example/docs/readme.md", root: "example", want: "docs/readme.md"},
		{name: "skip root dir entry", path: "example/", root: "example", want: ""},
		{name: "leave unmatched", path: "other/program", root: "example", want: "other/program"},
		{name: "leave when disabled", path: "example/program", root: "", want: "example/program"},
	}
	for _, tc := range cases {
		if got := stripSharedRoot(tc.path, tc.root); got != tc.want {
			t.Fatalf("%s: stripSharedRoot(%q, %q) = %q, want %q", tc.name, tc.path, tc.root, got, tc.want)
		}
	}
}
