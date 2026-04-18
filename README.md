# arklite

`arklite` is a fast, portable, CLI-first archive tool built for Linux and shipped as a single binary.

It detects formats by signature, not extension, supports stdin, and keeps broad format coverage by using internal handlers where practical and external tools where necessary.

## Highlights

- Single Go binary
- Linux-first, cross-buildable for macOS, Windows, and FreeBSD
- Signature-based detection
- `extract`, `list`, `create`, `modify`, `preview`, `search`, `test`
- ZIP and tar-family support without external tools
- External fallback for `rar`, `7z`, `iso`, `img`, `deb`, `rpm`
- Progress bars
- Fuzzy search
- JSON output for scripting

## Supported formats

Detected by signature:

- `zip`
- `tar`
- `gzip`
- `xz`
- `zstd`
- `rar`
- `7z`
- `iso`
- `img`
- `deb`
- `rpm`

Handled internally:

- `zip`
- `tar`
- `tar.gz`
- `tar.xz`
- `tar.zst`

Handled through external tools when available:

- `rar`
- `7z`
- `iso`
- `img`
- `deb`
- `rpm`

Preferred external tools:

- `7z`
- `bsdtar`
- `unrar` for RAR

If a required external tool is missing, `arklite` prints a clear error and does not install anything automatically.

## Build

Requirements:

- Go 1.22+

```bash
go mod tidy
go build -trimpath -ldflags="-s -w" -o arklite .
```

Cross-build examples:

```bash
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o arklite-linux-amd64 .
GOOS=darwin GOARCH=arm64 CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o arklite-darwin-arm64 .
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o arklite-windows-amd64.exe .
GOOS=freebsd GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o arklite-freebsd-amd64 .
```

## Install

Build the binary first, then install the current executable:

User-local install:

```bash
./arklite install --user
```

System-wide install:

```bash
sudo ./arklite install --system
```

Default install targets:

- Linux/macOS/FreeBSD user: `~/.local/bin`
- Linux/macOS/FreeBSD system: `/usr/local/bin`
- Windows user: `%LocalAppData%\Programs\arklite\bin`

After install, the binary can be run from any directory if the install directory is on `PATH`. `arklite install` warns when it is not.

## Usage

```bash
arklite extract [options] <archive|->
arklite list [options] <archive|->
arklite create --format <fmt> [options] <output> <input...>
arklite preview [options] <archive|->
arklite search [options] <archive|-> <query>
arklite modify [options] <archive>
arklite test <archive|->
arklite install [options]
arklite formats
```

Examples:

```bash
arklite list sample.bin
arklite extract --out ./out sample.bin
cat sample.bin | arklite list -
arklite create --format zip --level 9 out.bin ./folder
arklite create --format tar.zst --level 19 out.bin ./folder
arklite modify archive.bin --add ./newfile --remove '.*\.tmp$'
arklite preview --select conf archive.bin
arklite search archive.bin readme
arklite test archive.bin
```

## Notes

- Options must come before positional arguments.
- Extraction filters and `--dry-run` are available for internally handled formats.
- Tar symlink extraction is skipped on Windows when the platform does not allow it.
