package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func resolveInputArchive(path string) (string, func(), error) {
	if path != "-" {
		debugf("using archive path: %s", path)
		return path, func() {}, nil
	}
	tmp, err := os.CreateTemp("", "arklite-stdin-*.bin")
	if err != nil {
		return "", nil, err
	}
	if _, err := io.Copy(tmp, os.Stdin); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmp.Name())
		return "", nil, err
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmp.Name())
		return "", nil, err
	}
	debugf("buffered stdin archive to temporary file: %s", tmp.Name())
	return tmp.Name(), func() { _ = os.Remove(tmp.Name()) }, nil
}

func toolExists(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

func listExternal(path string, format Format) ([]Entry, error) {
	if toolExists("7z") {
		debugf("listing %s archive with 7z", format)
		out, err := exec.Command("7z", "l", "-ba", path).CombinedOutput()
		if err != nil {
			return nil, fmt.Errorf("7z list failed: %w\n%s", err, string(out))
		}
		entries := parse7zList(out)
		if len(entries) > 0 {
			return entries, nil
		}
	}
	if toolExists("bsdtar") {
		debugf("listing %s archive with bsdtar", format)
		out, err := exec.Command("bsdtar", "-tf", path).CombinedOutput()
		if err != nil {
			return nil, fmt.Errorf("bsdtar list failed: %w\n%s", err, string(out))
		}
		lines := strings.Split(strings.TrimSpace(string(out)), "\n")
		var entries []Entry
		for _, ln := range lines {
			ln = strings.TrimSpace(ln)
			if ln == "" {
				continue
			}
			entries = append(entries, Entry{Name: ln})
		}
		return entries, nil
	}
	if format == FmtRar && toolExists("unrar") {
		debugf("listing rar archive with unrar")
		out, err := exec.Command("unrar", "lb", path).CombinedOutput()
		if err != nil {
			return nil, fmt.Errorf("unrar list failed: %w\n%s", err, string(out))
		}
		lines := strings.Split(strings.TrimSpace(string(out)), "\n")
		var entries []Entry
		for _, ln := range lines {
			ln = strings.TrimSpace(ln)
			if ln == "" {
				continue
			}
			entries = append(entries, Entry{Name: ln})
		}
		return entries, nil
	}
	return nil, fmt.Errorf(
		"format %s requires external tool. missing one of: 7z, bsdtar%s",
		format,
		func() string {
			if format == FmtRar {
				return ", unrar"
			}
			return ""
		}(),
	)
}

func extractExternal(path, outDir string, format Format) error {
	if toolExists("7z") {
		debugf("extracting %s archive with 7z into %s", format, outDir)
		cmd := exec.Command("7z", "x", "-y", "-o"+outDir, path)
		cmd.Stdout = os.Stderr
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("7z extract failed: %w", err)
		}
		return nil
	}
	if toolExists("bsdtar") {
		debugf("extracting %s archive with bsdtar into %s", format, outDir)
		cmd := exec.Command("bsdtar", "-xf", path, "-C", outDir)
		cmd.Stdout = os.Stderr
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("bsdtar extract failed: %w", err)
		}
		return nil
	}
	if format == FmtRar && toolExists("unrar") {
		debugf("extracting rar archive with unrar into %s", outDir)
		cmd := exec.Command("unrar", "x", "-o+", path, filepath.Clean(outDir))
		cmd.Stdout = os.Stderr
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("unrar extract failed: %w", err)
		}
		return nil
	}
	return fmt.Errorf(
		"format %s requires external tool. missing one of: 7z, bsdtar%s",
		format,
		func() string {
			if format == FmtRar {
				return ", unrar"
			}
			return ""
		}(),
	)
}

func testExternal(path string, format Format) error {
	if toolExists("7z") {
		debugf("testing %s archive with 7z", format)
		out, err := exec.Command("7z", "t", path).CombinedOutput()
		if err != nil {
			return fmt.Errorf("7z test failed: %w\n%s", err, string(out))
		}
		return nil
	}
	if toolExists("bsdtar") {
		debugf("testing %s archive with bsdtar", format)
		out, err := exec.Command("bsdtar", "-tf", path).CombinedOutput()
		if err != nil {
			return fmt.Errorf("bsdtar test failed: %w\n%s", err, string(out))
		}
		return nil
	}
	if format == FmtRar && toolExists("unrar") {
		debugf("testing rar archive with unrar")
		out, err := exec.Command("unrar", "t", "-idq", path).CombinedOutput()
		if err != nil {
			return fmt.Errorf("unrar test failed: %w\n%s", err, string(out))
		}
		return nil
	}
	return fmt.Errorf(
		"format %s requires external tool. missing one of: 7z, bsdtar%s",
		format,
		func() string {
			if format == FmtRar {
				return ", unrar"
			}
			return ""
		}(),
	)
}

func parse7zList(out []byte) []Entry {
	sc := bufio.NewScanner(bytes.NewReader(out))
	var entries []Entry
	inTable := false
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if strings.HasPrefix(line, "----------") {
			inTable = !inTable
			continue
		}
		if !inTable || line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 6 {
			continue
		}
		name := strings.Join(fields[5:], " ")
		entries = append(entries, Entry{Name: name})
	}
	return entries
}
