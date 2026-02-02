# ogo

Simple HTTP file server in Go with OpenBSD security features (pledge/unveil).

**Note:** This is a development/learning project, not intended for production use.

## Build

```bash
go build -o ogo
```

## Usage

```bash
./ogo -d <directory> [-a <address>] [-t <timeout>] [-v]
```

### Flags

| Flag | Default | Description |
|------|---------|-------------|
| `-d` | `.` | Directory to serve |
| `-a` | `:8080` | Listen address |
| `-t` | `5s` | Shutdown timeout |
| `-v` | `false` | Debug logging |

### Example

```bash
./ogo -d /var/www -a :8080 -t 10s -v
```

## Linter

```sh
golangci-lint -c golangci.yml run
gosec ./...
govulncheck ./...
staticcheck ./...
```

## OpenBSD Security

On OpenBSD, uses `pledge` (stdio, rpath, inet, dns) and `unveil` (read-only access to served directory).

## License

See [LICENSE](./LICENSE) file.
