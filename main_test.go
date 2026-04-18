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
