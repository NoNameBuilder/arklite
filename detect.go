package main

import (
	"fmt"
	"io"
	"os"
)

type Format string

const (
	FmtUnknown Format = "unknown"
	FmtZip     Format = "zip"
	FmtTar     Format = "tar"
	FmtGzip    Format = "gzip"
	FmtXz      Format = "xz"
	FmtZstd    Format = "zstd"
	FmtRar     Format = "rar"
	Fmt7z      Format = "7z"
	FmtISO     Format = "iso"
	FmtIMG     Format = "img"
	FmtDEB     Format = "deb"
	FmtRPM     Format = "rpm"
)

func detectFormat(path string) (Format, error) {
	f, err := os.Open(path)
	if err != nil {
		return FmtUnknown, err
	}
	defer f.Close()

	head := make([]byte, 16384)
	n, err := io.ReadFull(f, head)
	if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
		return FmtUnknown, err
	}
	head = head[:n]

	stat, err := f.Stat()
	if err != nil {
		return FmtUnknown, err
	}

	return detectFromBytes(head, stat.Size())
}

func detectFromBytes(head []byte, size int64) (Format, error) {
	has := func(off int, sig []byte) bool {
		if len(head) < off+len(sig) {
			return false
		}
		for i := range sig {
			if head[off+i] != sig[i] {
				return false
			}
		}
		return true
	}

	switch {
	case has(0, []byte("PK\x03\x04")) || has(0, []byte("PK\x05\x06")) || has(0, []byte("PK\x07\x08")):
		return FmtZip, nil
	case has(0, []byte{0x1f, 0x8b}):
		return FmtGzip, nil
	case has(0, []byte{0xfd, '7', 'z', 'X', 'Z', 0x00}):
		return FmtXz, nil
	case has(0, []byte{0x28, 0xb5, 0x2f, 0xfd}):
		return FmtZstd, nil
	case has(0, []byte("Rar!\x1a\x07\x00")) || has(0, []byte("Rar!\x1a\x07\x01\x00")):
		return FmtRar, nil
	case has(0, []byte{'7', 'z', 0xbc, 0xaf, 0x27, 0x1c}):
		return Fmt7z, nil
	case has(0, []byte("!<arch>\n")):
		return FmtDEB, nil
	case has(0, []byte{0xed, 0xab, 0xee, 0xdb}):
		return FmtRPM, nil
	}

	if has(257, []byte("ustar\x00")) || has(257, []byte("ustar ")) {
		return FmtTar, nil
	}
	if has(0x8001, []byte("CD001")) {
		return FmtISO, nil
	}
	if size >= 512 && size%512 == 0 && len(head) >= 512 && head[510] == 0x55 && head[511] == 0xaa {
		return FmtIMG, nil
	}

	return FmtUnknown, nil
}

func mustDetect(path string) (Format, error) {
	f, err := detectFormat(path)
	if err != nil {
		return FmtUnknown, err
	}
	if f == FmtUnknown {
		return FmtUnknown, fmt.Errorf("could not detect archive format by signature")
	}
	return f, nil
}
