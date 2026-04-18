package main

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/klauspost/compress/zstd"
	"github.com/ulikunitz/xz"
)

type Entry struct {
	Name     string
	Size     int64
	Mode     fs.FileMode
	Modified string
	IsDir    bool
}

func runList(args []string) error {
	fs := flag.NewFlagSet("list", flag.ContinueOnError)
	filter := fs.String("filter", "", "regex filter on entry names")
	asJSON := fs.Bool("json", false, "print as JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return fmt.Errorf("usage: list [options] <archive|->")
	}
	src, cleanup, err := resolveInputArchive(fs.Arg(0))
	if err != nil {
		return err
	}
	defer cleanup()

	entries, format, err := listArchive(src)
	if err != nil {
		return err
	}
	var re *regexp.Regexp
	if *filter != "" {
		re, err = regexp.Compile(*filter)
		if err != nil {
			return fmt.Errorf("invalid filter regex: %w", err)
		}
	}
	if *asJSON {
		var filtered []Entry
		for _, e := range entries {
			if re != nil && !re.MatchString(e.Name) {
				continue
			}
			filtered = append(filtered, e)
		}
		out := struct {
			Format  Format  `json:"format"`
			Entries []Entry `json:"entries"`
		}{Format: format, Entries: filtered}
		return json.NewEncoder(os.Stdout).Encode(out)
	}
	fmt.Printf("Format: %s\n", format)
	for _, e := range entries {
		if re != nil && !re.MatchString(e.Name) {
			continue
		}
		dirMark := " "
		if e.IsDir {
			dirMark = "d"
		}
		fmt.Printf("%s %10d  %s\n", dirMark, e.Size, e.Name)
	}
	return nil
}

func runPreview(args []string) error {
	fs := flag.NewFlagSet("preview", flag.ContinueOnError)
	selectQuery := fs.String("select", "", "fuzzy-select query")
	limit := fs.Int("limit", 25, "max number of entries to print")
	asJSON := fs.Bool("json", false, "print as JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return fmt.Errorf("usage: preview [options] <archive|->")
	}
	src, cleanup, err := resolveInputArchive(fs.Arg(0))
	if err != nil {
		return err
	}
	defer cleanup()

	entries, format, err := listArchive(src)
	if err != nil {
		return err
	}
	names := make([]string, 0, len(entries))
	entryByName := make(map[string]Entry, len(entries))
	for _, e := range entries {
		names = append(names, e.Name)
		entryByName[e.Name] = e
	}

	var picked []string
	if *selectQuery == "" {
		picked = names
	} else {
		picked = fuzzyFilter(names, *selectQuery, *limit)
	}
	if *limit > 0 && len(picked) > *limit {
		picked = picked[:*limit]
	}
	if *asJSON {
		var filtered []Entry
		for _, n := range picked {
			filtered = append(filtered, entryByName[n])
		}
		out := struct {
			Format  Format  `json:"format"`
			Entries []Entry `json:"entries"`
		}{Format: format, Entries: filtered}
		return json.NewEncoder(os.Stdout).Encode(out)
	}
	fmt.Printf("Format: %s\n", format)

	for _, n := range picked {
		e := entryByName[n]
		fmt.Printf("%-10d %s\n", e.Size, e.Name)
	}
	return nil
}

func runSearch(args []string) error {
	fs := flag.NewFlagSet("search", flag.ContinueOnError)
	limit := fs.Int("limit", 50, "max hits")
	asJSON := fs.Bool("json", false, "print as JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 2 {
		return fmt.Errorf("usage: search [options] <archive|-> <query>")
	}
	src, cleanup, err := resolveInputArchive(fs.Arg(0))
	if err != nil {
		return err
	}
	defer cleanup()
	query := fs.Arg(1)

	entries, format, err := listArchive(src)
	if err != nil {
		return err
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		names = append(names, e.Name)
	}
	hits := fuzzyFilter(names, query, *limit)
	if *asJSON {
		out := struct {
			Format Format   `json:"format"`
			Query  string   `json:"query"`
			Hits   []string `json:"hits"`
		}{Format: format, Query: query, Hits: hits}
		return json.NewEncoder(os.Stdout).Encode(out)
	}
	fmt.Printf("Format: %s\n", format)
	for _, h := range hits {
		fmt.Println(h)
	}
	return nil
}

func runExtract(args []string) error {
	fs := flag.NewFlagSet("extract", flag.ContinueOnError)
	out := fs.String("out", ".", "output directory")
	flat := fs.Bool("flat", false, "extract to current dir without archive folder")
	threadsValue := fs.String("threads", "auto", "worker count for zip extraction: auto or positive integer")
	include := fs.String("include", "", "regex of entries to extract")
	exclude := fs.String("exclude", "", "regex of entries to skip")
	dryRun := fs.Bool("dry-run", false, "only print what would be extracted")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return fmt.Errorf("usage: extract [options] <archive|->")
	}
	src, cleanup, err := resolveInputArchive(fs.Arg(0))
	if err != nil {
		return err
	}
	defer cleanup()

	threads, err := parseThreads(*threadsValue)
	if err != nil {
		return err
	}

	outDir := sanitizeOutputDir(*out)
	if !*flat {
		base := archiveBaseName(fs.Arg(0))
		outDir = filepath.Join(outDir, base+"_out")
	}

	format, err := mustDetect(src)
	if err != nil {
		return err
	}
	debugf("detected archive format: %s", format)
	includeRe, err := compileRegexMaybe(*include)
	if err != nil {
		return fmt.Errorf("invalid include regex: %w", err)
	}
	excludeRe, err := compileRegexMaybe(*exclude)
	if err != nil {
		return fmt.Errorf("invalid exclude regex: %w", err)
	}
	allow := func(name string) bool {
		if includeRe != nil && !includeRe.MatchString(name) {
			return false
		}
		if excludeRe != nil && excludeRe.MatchString(name) {
			return false
		}
		return true
	}

	if *dryRun {
		outDir = ""
	} else if err := os.MkdirAll(outDir, 0o755); err != nil {
		return err
	}

	switch format {
	case FmtZip:
		return extractZIP(src, outDir, threads, allow, *dryRun)
	case FmtTar, FmtGzip, FmtXz, FmtZstd:
		err := extractTarFamily(src, outDir, format, allow, *dryRun)
		if err == nil {
			return nil
		}
		if *dryRun || includeRe != nil || excludeRe != nil {
			return fmt.Errorf("extract filters and dry-run are only available for internally handled formats; detected %s could not be handled internally", format)
		}
		return extractExternal(src, outDir, format)
	default:
		if *dryRun || includeRe != nil || excludeRe != nil {
			return fmt.Errorf("extract filters and dry-run are not available for format %s; install an internal-capable format or extract without filters", format)
		}
		return extractExternal(src, outDir, format)
	}
}

func listArchive(path string) ([]Entry, Format, error) {
	format, err := mustDetect(path)
	if err != nil {
		return nil, FmtUnknown, err
	}
	debugf("detected archive format: %s", format)

	switch format {
	case FmtZip:
		entries, err := listZIP(path)
		return entries, format, err
	case FmtTar, FmtGzip, FmtXz, FmtZstd:
		entries, err := listTarFamily(path, format)
		if err != nil {
			entries, err = listExternal(path, format)
		}
		return entries, format, err
	default:
		entries, err := listExternal(path, format)
		return entries, format, err
	}
}

func listZIP(path string) ([]Entry, error) {
	r, err := zip.OpenReader(path)
	if err != nil {
		return nil, err
	}
	defer r.Close()
	out := make([]Entry, 0, len(r.File))
	for _, f := range r.File {
		out = append(out, Entry{
			Name:     f.Name,
			Size:     int64(f.UncompressedSize64),
			Mode:     f.Mode(),
			Modified: f.Modified.UTC().Format(time.RFC3339),
			IsDir:    f.FileInfo().IsDir(),
		})
	}
	return out, nil
}

func listTarFamily(path string, format Format) ([]Entry, error) {
	rc, closeFn, err := openTarStream(path, format)
	if err != nil {
		return nil, err
	}
	defer closeFn()

	tr := tar.NewReader(rc)
	var out []Entry
	for {
		h, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, err
		}
		out = append(out, Entry{
			Name:     h.Name,
			Size:     h.Size,
			Mode:     fs.FileMode(h.Mode),
			Modified: h.ModTime.UTC().Format(time.RFC3339),
			IsDir:    h.FileInfo().IsDir(),
		})
	}
	return out, nil
}

func extractZIP(path, outDir string, threads int, allow func(string) bool, dryRun bool) error {
	r, err := zip.OpenReader(path)
	if err != nil {
		return err
	}
	defer r.Close()

	if threads < 1 {
		threads = 1
	}
	type task struct {
		f *zip.File
	}
	tasks := make(chan task, threads*2)
	errCh := make(chan error, 1)
	var wg sync.WaitGroup

	var total int64
	for _, f := range r.File {
		if allow != nil && !allow(f.Name) {
			continue
		}
		if !f.FileInfo().IsDir() {
			total += int64(f.UncompressedSize64)
		}
	}
	counter := &byteCounter{}
	p := startProgress("extract(zip)", total, counter)
	defer p.Finish()

	worker := func() {
		defer wg.Done()
		for t := range tasks {
			if e := extractOneZIPFile(t.f, outDir, counter); e != nil {
				select {
				case errCh <- e:
				default:
				}
				return
			}
		}
	}
	for i := 0; i < threads; i++ {
		wg.Add(1)
		go worker()
	}
	for _, f := range r.File {
		if allow != nil && !allow(f.Name) {
			continue
		}
		if dryRun {
			fmt.Println(f.Name)
			continue
		}
		select {
		case e := <-errCh:
			close(tasks)
			wg.Wait()
			return e
		default:
		}
		tasks <- task{f: f}
	}
	close(tasks)
	wg.Wait()
	select {
	case e := <-errCh:
		return e
	default:
		return nil
	}
}

func extractOneZIPFile(f *zip.File, outDir string, counter *byteCounter) error {
	target, err := secureJoin(outDir, f.Name)
	if err != nil {
		return err
	}
	if f.FileInfo().IsDir() {
		return os.MkdirAll(target, 0o755)
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	src, err := f.Open()
	if err != nil {
		return err
	}
	defer src.Close()

	dst, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, f.Mode())
	if err != nil {
		return err
	}
	defer dst.Close()

	_, err = io.Copy(dst, &countingReader{r: src, c: counter})
	return err
}

func extractTarFamily(path, outDir string, format Format, allow func(string) bool, dryRun bool) error {
	rc, closeFn, err := openTarStream(path, format)
	if err != nil {
		return err
	}
	defer closeFn()

	stat, _ := os.Stat(path)
	total := int64(0)
	if stat != nil {
		total = stat.Size()
	}
	counter := &byteCounter{}
	p := startProgress("extract(tar)", total, counter)
	defer p.Finish()

	tr := tar.NewReader(&countingReader{r: rc, c: counter})
	for {
		h, err := tr.Next()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
		if allow != nil && !allow(h.Name) {
			continue
		}
		if dryRun {
			fmt.Println(h.Name)
			if h.Typeflag == tar.TypeReg || h.Typeflag == tar.TypeRegA {
				if _, err := io.Copy(io.Discard, tr); err != nil {
					return err
				}
			}
			continue
		}
		target, err := secureJoin(outDir, h.Name)
		if err != nil {
			return err
		}
		switch h.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
		case tar.TypeReg, tar.TypeRegA:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			dst, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, fs.FileMode(h.Mode))
			if err != nil {
				return err
			}
			_, cpErr := io.Copy(dst, tr)
			closeErr := dst.Close()
			if cpErr != nil {
				return cpErr
			}
			if closeErr != nil {
				return closeErr
			}
		case tar.TypeSymlink:
			if runtime.GOOS == "windows" {
				continue
			}
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			if err := os.Symlink(h.Linkname, target); err != nil && !os.IsExist(err) {
				return err
			}
		default:
		}
	}
}

func compileRegexMaybe(v string) (*regexp.Regexp, error) {
	if strings.TrimSpace(v) == "" {
		return nil, nil
	}
	return regexp.Compile(v)
}

func runTest(args []string) error {
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return fmt.Errorf("usage: test <archive|->")
	}
	src, cleanup, err := resolveInputArchive(fs.Arg(0))
	if err != nil {
		return err
	}
	defer cleanup()
	format, err := mustDetect(src)
	if err != nil {
		return err
	}
	debugf("detected archive format: %s", format)
	switch format {
	case FmtZip:
		r, err := zip.OpenReader(src)
		if err != nil {
			return err
		}
		defer r.Close()
		for _, f := range r.File {
			if f.FileInfo().IsDir() {
				continue
			}
			rc, err := f.Open()
			if err != nil {
				return err
			}
			_, err = io.Copy(io.Discard, rc)
			_ = rc.Close()
			if err != nil {
				return err
			}
		}
	case FmtTar, FmtGzip, FmtXz, FmtZstd:
		rc, closeFn, err := openTarStream(src, format)
		if err != nil {
			return err
		}
		defer closeFn()
		tr := tar.NewReader(rc)
		for {
			h, err := tr.Next()
			if errors.Is(err, io.EOF) {
				break
			}
			if err != nil {
				return err
			}
			if h.Typeflag == tar.TypeReg || h.Typeflag == tar.TypeRegA {
				if _, err := io.Copy(io.Discard, tr); err != nil {
					return err
				}
			}
		}
	default:
		if err := testExternal(src, format); err != nil {
			return err
		}
	}
	fmt.Printf("OK: %s (%s)\n", src, format)
	return nil
}

func openTarStream(path string, format Format) (io.Reader, func() error, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, nil, err
	}
	closeFn := f.Close
	switch format {
	case FmtTar:
		return f, closeFn, nil
	case FmtGzip:
		gz, err := gzip.NewReader(f)
		if err != nil {
			_ = f.Close()
			return nil, nil, err
		}
		return gz, func() error {
			_ = gz.Close()
			return f.Close()
		}, nil
	case FmtXz:
		xzr, err := xz.NewReader(f)
		if err != nil {
			_ = f.Close()
			return nil, nil, err
		}
		return xzr, closeFn, nil
	case FmtZstd:
		zr, err := zstd.NewReader(f)
		if err != nil {
			_ = f.Close()
			return nil, nil, err
		}
		return zr, func() error {
			zr.Close()
			return f.Close()
		}, nil
	default:
		_ = f.Close()
		return nil, nil, fmt.Errorf("not a tar-family format: %s", format)
	}
}

func secureJoin(root, name string) (string, error) {
	cleanName := filepath.Clean(name)
	if cleanName == "." {
		return root, nil
	}
	target := filepath.Join(root, cleanName)
	rel, err := filepath.Rel(root, target)
	if err != nil {
		return "", err
	}
	if strings.HasPrefix(rel, "..") || strings.Contains(rel, "../") {
		return "", fmt.Errorf("path traversal blocked: %s", name)
	}
	return target, nil
}

func archiveBaseName(path string) string {
	base := filepath.Base(path)
	base = strings.TrimSpace(base)
	if base == "-" {
		return "archive"
	}
	base = strings.TrimSuffix(base, filepath.Ext(base))
	base = strings.TrimSuffix(base, filepath.Ext(base))
	if base == "" || base == "." || base == string(filepath.Separator) {
		return "archive"
	}
	return base
}
