package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

func runInstall(args []string) error {
	fs := flag.NewFlagSet("install", flag.ContinueOnError)
	system := fs.Bool("system", false, "install to a system-wide bin directory")
	user := fs.Bool("user", false, "install to a user-local bin directory")
	dir := fs.String("dir", "", "explicit install directory")
	name := fs.String("name", binaryName(), "installed binary name")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return fmt.Errorf("usage: install [--system|--user|--dir <path>] [--name <binary>]")
	}
	if *system && *user {
		return fmt.Errorf("--system and --user are mutually exclusive")
	}

	targetDir := *dir
	if targetDir == "" {
		switch {
		case *system:
			targetDir = defaultSystemInstallDir()
		default:
			targetDir = defaultUserInstallDir()
		}
	}
	if strings.TrimSpace(targetDir) == "" {
		return fmt.Errorf("could not determine install directory")
	}

	src, err := os.Executable()
	if err != nil {
		return err
	}
	src, err = filepath.EvalSymlinks(src)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		return err
	}
	dst := filepath.Join(targetDir, *name)
	if err := copyExecutable(src, dst); err != nil {
		return err
	}

	fmt.Printf("Installed: %s\n", dst)
	if !pathContainsDir(targetDir) {
		fmt.Printf("PATH update required: add %s to PATH\n", targetDir)
	}
	return nil
}

func binaryName() string {
	if runtime.GOOS == "windows" {
		return "arklite.exe"
	}
	return "arklite"
}

func defaultSystemInstallDir() string {
	if runtime.GOOS == "windows" {
		base := os.Getenv("ProgramFiles")
		if base == "" {
			base = `C:\Program Files`
		}
		return filepath.Join(base, "arklite")
	}
	return "/usr/local/bin"
}

func defaultUserInstallDir() string {
	if runtime.GOOS == "windows" {
		base := os.Getenv("LocalAppData")
		if base == "" {
			base = filepath.Join(os.Getenv("USERPROFILE"), "AppData", "Local")
		}
		return filepath.Join(base, "Programs", "arklite", "bin")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".local", "bin")
}

func copyExecutable(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	tmp, err := os.CreateTemp(filepath.Dir(dst), ".arklite-install-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	success := false
	defer func() {
		_ = tmp.Close()
		if !success {
			_ = os.Remove(tmpName)
		}
	}()

	if _, err := io.Copy(tmp, in); err != nil {
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}

	mode := os.FileMode(0o755)
	if runtime.GOOS == "windows" {
		mode = 0o644
	}
	if err := os.Chmod(tmpName, mode); err != nil && !errors.Is(err, os.ErrPermission) {
		return err
	}
	if err := replaceFile(tmpName, dst); err != nil {
		return err
	}
	success = true
	return nil
}

func replaceFile(src, dst string) error {
	if runtime.GOOS == "windows" {
		_ = os.Remove(dst)
	}
	return os.Rename(src, dst)
}

func pathContainsDir(dir string) bool {
	pathValue := os.Getenv("PATH")
	for _, entry := range filepath.SplitList(pathValue) {
		if samePath(entry, dir) {
			return true
		}
	}
	return false
}

func samePath(a, b string) bool {
	ca := filepath.Clean(a)
	cb := filepath.Clean(b)
	if runtime.GOOS == "windows" {
		return strings.EqualFold(ca, cb)
	}
	return ca == cb
}
