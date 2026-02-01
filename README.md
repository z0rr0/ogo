# ogo
Simple file server for OpenBSD written on Go

## Description

`ogo` is a lightweight file server written in Go that serves files from a specified directory over HTTP. It features graceful shutdown handling and OpenBSD-specific security enhancements using `pledge` and `unveil` system calls.

## Features

- Serves files from any directory via HTTP
- Configurable shutdown timeout for graceful termination
- OpenBSD security: Uses `pledge` and `unveil` system calls when built on OpenBSD
- Simple command-line interface
- Based on Go's standard library `http.FileServer`

## Installation

### Prerequisites

- Go 1.17 or later

### Build

```bash
go build -o ogo
```

### Build for OpenBSD

When building on OpenBSD, the security features (`pledge` and `unveil`) are automatically enabled:

```bash
go build -o ogo
```

## Usage

```bash
./ogo -d <directory> [-t <timeout>]
```

### Flags

- `-d` (required): Directory to serve files from
- `-t` (optional): Shutdown timeout duration (default: 5s)

### Examples

Serve files from the current directory:
```bash
./ogo -d .
```

Serve files from `/var/www` with a 10-second shutdown timeout:
```bash
./ogo -d /var/www -t 10s
```

## Server Details

- **Default Port**: 8080
- **Protocol**: HTTP
- **Access**: `http://localhost:8080/`

## Security (OpenBSD)

When running on OpenBSD, `ogo` applies additional security restrictions:

- **unveil**: Restricts filesystem access to only the specified directory
- **pledge**: Limits system calls to:
  - `stdio`: Standard I/O operations
  - `rpath`: Read filesystem paths
  - `inet`: Network operations
  - `dns`: DNS lookups

## Graceful Shutdown

The server handles interrupt signals (`SIGINT`, `SIGTERM`) gracefully:
1. Receives shutdown signal
2. Stops accepting new connections
3. Waits for active connections to complete (up to timeout duration)
4. Exits cleanly

## License

See LICENSE file for details.

