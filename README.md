# arklite

`arklite` is a small command-line archive tool.

It can:

- list files in an archive
- extract files
- create archives
- modify `zip` and tar-based archives
- search file names
- test archive integrity

It detects archive type by file signature, not by file extension.

## Why use arklite

Use `arklite` if you want:

- one small CLI binary instead of a full GUI archive manager
- the same commands in scripts, terminals, SSH sessions, and containers
- signature-based detection instead of trusting file extensions
- stdin support, fuzzy search, JSON output, and fast basic archive workflows
- a simpler interface than mixing `tar`, `zip`, `unzip`, `7z`, and other tools by hand

Compared with KDE Ark:

- `arklite` is CLI-first
- `arklite` fits headless systems and automation better
- `arklite` is easier to ship as a single binary

Compared with raw archive tools:

- `arklite` gives one command layout across formats
- `arklite` has cleaner defaults and clearer errors
- `arklite` avoids extension-based guessing

If you want a full desktop archive manager with previews and GUI integration, use Ark. If you want a terminal tool with one consistent interface, use `arklite`.

## Building it is preferred over using the precompiled binary.
- Takes only ~2 seconds.

## Build

Needs Go `1.22+`.

```bash
go mod tidy
go build -trimpath -ldflags="-s -w" -o arklite .
```

## Install

User install:

```bash
./arklite install --user
```

System install:

```bash
sudo ./arklite install --system
```

After install, you can run `arklite` from any directory if the install path is in `PATH`.

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

Global option:

```bash
arklite --verbose <command> ...
```

## Examples

```bash
arklite list sample.bin
arklite extract --out out sample.bin
cat sample.bin | arklite list -
arklite create --format zip --level 9 out.bin folder
arklite create --format tar.zst --level 19 out.bin folder
arklite create --format tar.zst --threads auto out.bin folder
arklite extract --threads 2 archive.bin
arklite extract --auto-root example.zip
arklite modify archive.bin --add newfile --remove '.*\.tmp$'
arklite preview --select conf archive.bin
arklite search archive.bin readme
arklite test archive.bin
```

`arklite extract --auto-root example.zip` keeps the default output folder name `example`, but if the archive already contains a single shared top-level folder like `example/program`, it strips that layer and extracts to `example/program`.

## Formats

Built in:

- `zip`
- `tar`
- `tar.gz`
- `tar.xz`
- `tar.zst`

Also detected:

- `rar`
- `7z`
- `iso`
- `img`
- `deb`
- `rpm`

Some non-built-in formats need external tools.
