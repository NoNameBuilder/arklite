# arklite

`arklite` is a relatively small (~3.5MB once compiled) command-line archive tool.

It can:

- list files in an archive
- extract files
- create archives
- modify `zip` and tar-based archives
- search file names
- test archive integrity

It detects archive type by file signature, not by file extension.

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

## Examples

```bash
arklite list sample.bin
arklite extract --out out sample.bin
cat sample.bin | arklite list -
arklite create --format zip --level 9 out.bin folder
arklite create --format tar.zst --level 19 out.bin folder
arklite modify archive.bin --add newfile --remove '.*\.tmp$'
arklite preview --select conf archive.bin
arklite search archive.bin readme
arklite test archive.bin
```

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
