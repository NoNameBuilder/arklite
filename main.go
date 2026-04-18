package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(2)
	}
	cmd := os.Args[1]
	args := os.Args[2:]

	var err error
	switch cmd {
	case "extract":
		err = runExtract(args)
	case "list":
		err = runList(args)
	case "create":
		err = runCreate(args)
	case "preview":
		err = runPreview(args)
	case "search":
		err = runSearch(args)
	case "modify":
		err = runModify(args)
	case "test":
		err = runTest(args)
	case "install":
		err = runInstall(args)
	case "formats":
		printFormats()
		return
	case "version":
		fmt.Println("arklite 0.1.0")
		return
	case "help", "-h", "--help":
		printUsage()
		return
	default:
		printUsage()
		fmt.Fprintf(os.Stderr, "\nunknown command: %s\n", cmd)
		os.Exit(2)
	}
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return
		}
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func printUsage() {
	exe := filepath.Base(os.Args[0])
	fmt.Printf(`%s: portable archive CLI

Usage:
  %s extract [options] <archive|-> 
  %s list [options] <archive|->
  %s create --format <fmt> [options] <output> <input...>
  %s preview [options] <archive|->
  %s search [options] <archive|-> <query>
  %s modify [options] <archive>
  %s test [options] <archive|->
  %s install [options]
  %s formats

Commands:
  extract    Extract archive contents
  list       List archive entries
  create     Create archive from files/dirs
  preview    List + optionally select entries (metadata only)
  search     Fuzzy search file names in archive
  modify     Add/remove files (zip/tar family)
  test       Verify archive integrity/readability
  install    Install the current binary into PATH
  formats    Show supported formats

Examples:
  %s extract a.bin --out outdir
  cat archive.bin | %s list -
  %s create --format zip out.any ./folder
  %s create --format zip --level 9 out.any ./folder
  %s create --format tar.zst out.any ./folder
  %s modify a.any --add ./newfile --remove '.*\\.tmp$'
  %s test archive.any
  %s install --user
  sudo %s install --system

Platform:
  Primary: Linux
  Also supported: Windows, macOS, FreeBSD (best effort, external tools may vary)
`, exe, exe, exe, exe, exe, exe, exe, exe, exe, exe, exe, exe, exe, exe, exe, exe, exe, exe, exe)
}

func printFormats() {
	fmt.Println("Detected by magic bytes:")
	fmt.Println("  zip, tar, gzip, xz, zstd, rar, 7z, iso, img, deb, rpm")
	fmt.Println("Internal (no external tool needed):")
	fmt.Println("  zip, tar, tar.gz, tar.xz, tar.zst, gzip, xz, zstd")
	fmt.Println("External fallback (tool required):")
	fmt.Println("  rar, 7z, iso, img, deb, rpm (tries 7z/bsdtar/unrar/ar/rpm2cpio)")
	fmt.Printf("Runtime: %s/%s\n", runtime.GOOS, runtime.GOARCH)
}

func defaultThreads() int {
	n := runtime.NumCPU()
	if n < 2 {
		return 2
	}
	if n > 16 {
		return 16
	}
	return n
}

func sanitizeOutputDir(path string) string {
	if strings.TrimSpace(path) == "" {
		return "."
	}
	return path
}
