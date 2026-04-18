package main

import (
	"archive/tar"
	"archive/zip"
	"compress/flate"
	"compress/gzip"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/klauspost/compress/zstd"
	"github.com/ulikunitz/xz"
)

func runCreate(args []string) error {
	fs := flag.NewFlagSet("create", flag.ContinueOnError)
	formatStr := fs.String("format", "", "archive format: zip|tar|tar.gz|tar.xz|tar.zst")
	threads := fs.Int("threads", defaultThreads(), "compression threads for tar.zst")
	level := fs.Int("level", -1, "compression level (zip/gz: 0-9, zst: 1-22; -1 default)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() < 2 {
		return fmt.Errorf("usage: create --format <fmt> <output> <input...>")
	}
	if *formatStr == "" {
		return fmt.Errorf("--format is required (tool does not use extension-based behavior)")
	}
	outPath := fs.Arg(0)
	inputs := fs.Args()[1:]

	switch strings.ToLower(*formatStr) {
	case "zip":
		return createZIP(outPath, inputs, *level)
	case "tar":
		if *level != -1 {
			return fmt.Errorf("--level is not used for tar")
		}
		return createTar(outPath, inputs, "none", *threads)
	case "tar.gz", "targz", "tgz":
		return createTar(outPath, inputs, "gz", *threads, *level)
	case "tar.xz", "tarxz", "txz":
		if *level != -1 {
			return fmt.Errorf("--level is not currently supported for tar.xz")
		}
		return createTar(outPath, inputs, "xz", *threads)
	case "tar.zst", "tarzst", "tzst":
		return createTar(outPath, inputs, "zst", *threads, *level)
	default:
		return fmt.Errorf("unsupported create format %q", *formatStr)
	}
}

func createZIP(outPath string, inputs []string, level int) error {
	if level < -1 || level > 9 {
		return fmt.Errorf("zip compression level must be -1 or 0..9")
	}
	if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil && filepath.Dir(outPath) != "." {
		return err
	}
	out, err := os.Create(outPath)
	if err != nil {
		return err
	}
	defer out.Close()

	zw := zip.NewWriter(out)
	defer zw.Close()
	if level >= 0 {
		zw.RegisterCompressor(zip.Deflate, func(w io.Writer) (io.WriteCloser, error) {
			return flate.NewWriter(w, level)
		})
	}

	total, _ := totalInputSize(inputs)
	counter := &byteCounter{}
	p := startProgress("create(zip)", total, counter)
	defer p.Finish()

	for _, root := range inputs {
		if err := addPathToZIP(zw, root, "", counter); err != nil {
			return err
		}
	}
	return nil
}

func addPathToZIP(zw *zip.Writer, path, base string, counter *byteCounter) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if base == "" {
		base = filepath.Dir(path)
	}
	if info.IsDir() {
		return filepath.WalkDir(path, func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			rel, err := filepath.Rel(base, p)
			if err != nil {
				return err
			}
			rel = filepath.ToSlash(rel)
			if d.IsDir() {
				if rel == "." {
					return nil
				}
				_, err := zw.Create(rel + "/")
				return err
			}
			return writeFileToZIP(zw, p, rel, counter)
		})
	}
	rel := filepath.Base(path)
	return writeFileToZIP(zw, path, rel, counter)
}

func writeFileToZIP(zw *zip.Writer, srcPath, name string, counter *byteCounter) error {
	info, err := os.Stat(srcPath)
	if err != nil {
		return err
	}
	h, err := zip.FileInfoHeader(info)
	if err != nil {
		return err
	}
	h.Name = filepath.ToSlash(name)
	h.Method = zip.Deflate
	w, err := zw.CreateHeader(h)
	if err != nil {
		return err
	}
	f, err := os.Open(srcPath)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.Copy(w, &countingReader{r: f, c: counter})
	return err
}

func createTar(outPath string, inputs []string, compression string, threads int, levels ...int) error {
	level := -1
	if len(levels) > 0 {
		level = levels[0]
	}
	if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil && filepath.Dir(outPath) != "." {
		return err
	}
	out, err := os.Create(outPath)
	if err != nil {
		return err
	}
	defer out.Close()

	var tw *tar.Writer
	var closer io.Closer
	switch compression {
	case "none":
		tw = tar.NewWriter(out)
	case "gz":
		var gw *gzip.Writer
		if level >= 0 {
			if level > 9 {
				return fmt.Errorf("gzip compression level must be -1 or 0..9")
			}
			gw, err = gzip.NewWriterLevel(out, level)
			if err != nil {
				return err
			}
		} else {
			gw = gzip.NewWriter(out)
		}
		closer = gw
		tw = tar.NewWriter(gw)
	case "xz":
		xw, err := xz.NewWriter(out)
		if err != nil {
			return err
		}
		closer = xw
		tw = tar.NewWriter(xw)
	case "zst":
		if threads < 1 {
			threads = 1
		}
		opts := []zstd.EOption{zstd.WithEncoderConcurrency(threads)}
		if level >= 0 {
			if level < 1 || level > 22 {
				return fmt.Errorf("zstd compression level must be -1 or 1..22")
			}
			opts = append(opts, zstd.WithEncoderLevel(zstd.EncoderLevelFromZstd(level)))
		}
		zw, err := zstd.NewWriter(out, opts...)
		if err != nil {
			return err
		}
		closer = zw
		tw = tar.NewWriter(zw)
	default:
		return fmt.Errorf("unsupported compression %q", compression)
	}
	defer tw.Close()
	if closer != nil {
		defer closer.Close()
	}

	total, _ := totalInputSize(inputs)
	counter := &byteCounter{}
	p := startProgress("create(tar)", total, counter)
	defer p.Finish()

	for _, root := range inputs {
		if err := addPathToTar(tw, root, "", counter); err != nil {
			return err
		}
	}
	return nil
}

func addPathToTar(tw *tar.Writer, path, base string, counter *byteCounter) error {
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if base == "" {
		base = filepath.Dir(path)
	}
	if info.IsDir() {
		return filepath.Walk(path, func(p string, fi fs.FileInfo, err error) error {
			if err != nil {
				return err
			}
			rel, err := filepath.Rel(base, p)
			if err != nil {
				return err
			}
			rel = filepath.ToSlash(rel)
			if rel == "." {
				return nil
			}
			h, err := tar.FileInfoHeader(fi, "")
			if err != nil {
				return err
			}
			h.Name = rel
			if err := tw.WriteHeader(h); err != nil {
				return err
			}
			if fi.Mode().IsRegular() {
				f, err := os.Open(p)
				if err != nil {
					return err
				}
				_, err = io.Copy(tw, &countingReader{r: f, c: counter})
				_ = f.Close()
				if err != nil {
					return err
				}
			}
			return nil
		})
	}
	return writeFileToTar(tw, path, filepath.Base(path), counter)
}

func writeFileToTar(tw *tar.Writer, path, name string, counter *byteCounter) error {
	fi, err := os.Stat(path)
	if err != nil {
		return err
	}
	h, err := tar.FileInfoHeader(fi, "")
	if err != nil {
		return err
	}
	h.Name = filepath.ToSlash(name)
	if err := tw.WriteHeader(h); err != nil {
		return err
	}
	if !fi.Mode().IsRegular() {
		return nil
	}
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.Copy(tw, &countingReader{r: f, c: counter})
	return err
}

func totalInputSize(inputs []string) (int64, error) {
	var total int64
	for _, p := range inputs {
		err := filepath.Walk(p, func(_ string, info fs.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if info.Mode().IsRegular() {
				total += info.Size()
			}
			return nil
		})
		if err != nil {
			return total, err
		}
	}
	return total, nil
}

func runModify(args []string) error {
	fs := flag.NewFlagSet("modify", flag.ContinueOnError)
	addVals := strSliceFlag{}
	remVals := strSliceFlag{}
	fs.Var(&addVals, "add", "file/dir path to add (repeatable)")
	fs.Var(&remVals, "remove", "regex of entries to remove (repeatable)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return fmt.Errorf("usage: modify [options] <archive>")
	}
	if len(addVals) == 0 && len(remVals) == 0 {
		return fmt.Errorf("nothing to do: provide --add and/or --remove")
	}
	archive := fs.Arg(0)
	format, err := mustDetect(archive)
	if err != nil {
		return err
	}

	var rem []*regexp.Regexp
	for _, pat := range remVals {
		r, err := regexp.Compile(pat)
		if err != nil {
			return fmt.Errorf("invalid remove regex %q: %w", pat, err)
		}
		rem = append(rem, r)
	}

	switch format {
	case FmtZip:
		return modifyZIP(archive, addVals, rem)
	case FmtTar, FmtGzip, FmtXz, FmtZstd:
		return modifyTarFamily(archive, format, addVals, rem)
	default:
		return fmt.Errorf("modify is currently implemented for zip and tar-family formats; detected: %s", format)
	}
}

func modifyZIP(path string, adds []string, rem []*regexp.Regexp) error {
	src, err := zip.OpenReader(path)
	if err != nil {
		return err
	}
	defer src.Close()

	tmp := path + ".tmp"
	out, err := os.Create(tmp)
	if err != nil {
		return err
	}
	defer os.Remove(tmp)
	zw := zip.NewWriter(out)

	shouldKeep := func(name string) bool {
		for _, r := range rem {
			if r.MatchString(name) {
				return false
			}
		}
		return true
	}

	for _, f := range src.File {
		if !shouldKeep(f.Name) {
			continue
		}
		h := f.FileHeader
		w, err := zw.CreateHeader(&h)
		if err != nil {
			_ = zw.Close()
			_ = out.Close()
			return err
		}
		if f.FileInfo().IsDir() {
			continue
		}
		r, err := f.Open()
		if err != nil {
			_ = zw.Close()
			_ = out.Close()
			return err
		}
		_, err = io.Copy(w, r)
		_ = r.Close()
		if err != nil {
			_ = zw.Close()
			_ = out.Close()
			return err
		}
	}
	for _, p := range adds {
		if err := addPathToZIP(zw, p, "", &byteCounter{}); err != nil {
			_ = zw.Close()
			_ = out.Close()
			return err
		}
	}

	if err := zw.Close(); err != nil {
		_ = out.Close()
		return err
	}
	if err := out.Close(); err != nil {
		return err
	}
	return replaceFile(tmp, path)
}

func modifyTarFamily(path string, format Format, adds []string, rem []*regexp.Regexp) error {
	tmp := path + ".tmp"
	inR, closeIn, err := openTarStream(path, format)
	if err != nil {
		return err
	}
	defer closeIn()

	out, err := os.Create(tmp)
	if err != nil {
		return err
	}
	defer out.Close()
	defer os.Remove(tmp)

	var tw *tar.Writer
	var closeOut func() error
	switch format {
	case FmtTar:
		tw = tar.NewWriter(out)
		closeOut = func() error { return nil }
	case FmtGzip:
		gw := gzip.NewWriter(out)
		tw = tar.NewWriter(gw)
		closeOut = gw.Close
	case FmtXz:
		xw, err := xz.NewWriter(out)
		if err != nil {
			return err
		}
		tw = tar.NewWriter(xw)
		closeOut = xw.Close
	case FmtZstd:
		zw, err := zstd.NewWriter(out)
		if err != nil {
			return err
		}
		tw = tar.NewWriter(zw)
		closeOut = func() error { zw.Close(); return nil }
	default:
		return fmt.Errorf("unsupported tar family format %s", format)
	}

	shouldKeep := func(name string) bool {
		for _, r := range rem {
			if r.MatchString(name) {
				return false
			}
		}
		return true
	}

	tr := tar.NewReader(inR)
	for {
		h, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			_ = tw.Close()
			return err
		}
		if !shouldKeep(h.Name) {
			continue
		}
		if err := tw.WriteHeader(h); err != nil {
			_ = tw.Close()
			return err
		}
		if h.Typeflag == tar.TypeReg || h.Typeflag == tar.TypeRegA {
			if _, err := io.Copy(tw, tr); err != nil {
				_ = tw.Close()
				return err
			}
		}
	}
	for _, p := range adds {
		if err := addPathToTar(tw, p, "", &byteCounter{}); err != nil {
			_ = tw.Close()
			return err
		}
	}
	if err := tw.Close(); err != nil {
		return err
	}
	if err := closeOut(); err != nil {
		return err
	}
	return replaceFile(tmp, path)
}

type strSliceFlag []string

func (s *strSliceFlag) String() string {
	return strings.Join(*s, ",")
}

func (s *strSliceFlag) Set(v string) error {
	*s = append(*s, v)
	return nil
}
