# Raft-backed Distributed Key-Value Store

A simple, usable distributed key-value store built on the Raft consensus algorithm with real TCP RPC (Go's `net/rpc`) and durable disk persistence.

- Production-style binaries: `kvserver` (server) and `kvclient` (CLI)
- Leader election and replication via Raft
- Disk-backed Raft state (atomic writes)
- Client retries with leader stickiness
- File upload/download helpers in CLI: `putfile` / `getfile`

---

## Quick start

### Windows (PowerShell)

1) Build binaries

```powershell
mkdir .\bin
go build -o .\bin\kvserver.exe .\cmd\kvserver
go build -o .\bin\kvclient.exe .\cmd\kvclient
```

2) Start a 3-node cluster (open three terminals)

Terminal 1:
```powershell
.\bin\kvserver.exe -id 0 -addr 127.0.0.1:8000 -peers "127.0.0.1:8000,127.0.0.1:8001,127.0.0.1:8002" -data-dir data
```

Terminal 2:
```powershell
.\bin\kvserver.exe -id 1 -addr 127.0.0.1:8001 -peers "127.0.0.1:8000,127.0.0.1:8001,127.0.0.1:8002" -data-dir data
```

Terminal 3:
```powershell
.\bin\kvserver.exe -id 2 -addr 127.0.0.1:8002 -peers "127.0.0.1:8000,127.0.0.1:8001,127.0.0.1:8002" -data-dir data
```

3) Use the client

```powershell
# basic KV
.\bin\kvclient.exe -servers "127.0.0.1:8000,127.0.0.1:8001,127.0.0.1:8002" put mykey "hello"
.\bin\kvclient.exe -servers "127.0.0.1:8000,127.0.0.1:8001,127.0.0.1:8002" append mykey " world"
.\bin\kvclient.exe -servers "127.0.0.1:8000,127.0.0.1:8001,127.0.0.1:8002" get mykey

# file helpers (base64 under the hood)
.\bin\kvclient.exe -servers "127.0.0.1:8000,127.0.0.1:8001,127.0.0.1:8002" putfile file:photo .\photo.jpg
.\bin\kvclient.exe -servers "127.0.0.1:8000,127.0.0.1:8001,127.0.0.1:8002" getfile file:photo .\restored.jpg
```

### Linux/macOS (bash/zsh)

1) Build binaries

```bash
mkdir -p ./bin
go build -o ./bin/kvserver ./cmd/kvserver
go build -o ./bin/kvclient ./cmd/kvclient
```

2) Start a 3-node cluster (open three terminals)

Terminal 1:
```bash
./bin/kvserver -id 0 -addr 127.0.0.1:8000 -peers "127.0.0.1:8000,127.0.0.1:8001,127.0.0.1:8002" -data-dir data
```

Terminal 2:
```bash
./bin/kvserver -id 1 -addr 127.0.0.1:8001 -peers "127.0.0.1:8000,127.0.0.1:8001,127.0.0.1:8002" -data-dir data
```

Terminal 3:
```bash
./bin/kvserver -id 2 -addr 127.0.0.1:8002 -peers "127.0.0.1:8000,127.0.0.1:8001,127.0.0.1:8002" -data-dir data
```

3) Use the client

```bash
# basic KV
./bin/kvclient -servers "127.0.0.1:8000,127.0.0.1:8001,127.0.0.1:8002" put mykey "hello"
./bin/kvclient -servers "127.0.0.1:8000,127.0.0.1:8001,127.0.0.1:8002" append mykey " world"
./bin/kvclient -servers "127.0.0.1:8000,127.0.0.1:8001,127.0.0.1:8002" get mykey

# file helpers (base64 under the hood)
./bin/kvclient -servers "127.0.0.1:8000,127.0.0.1:8001,127.0.0.1:8002" putfile file:photo ./photo.jpg
./bin/kvclient -servers "127.0.0.1:8000,127.0.0.1:8001,127.0.0.1:8002" getfile file:photo ./restored.jpg
```

---

## Flags and commands

### kvserver

- `-id` (int): This server's index in the peers list (0-based).
- `-addr` (host:port): TCP address to listen on. Must also appear in `-peers` at the same index.
- `-peers` (csv): Comma-separated list of all server addresses (including self). Order defines server IDs.
- `-data-dir` (path): Base directory for Raft state and snapshots. Server stores under `<data-dir>-<id>/`.

Example:
```bash
./kvserver -id 2 -addr 10.0.0.5:9002 -peers "10.0.0.3:9000,10.0.0.4:9001,10.0.0.5:9002" -data-dir data
```

### kvclient

Subcommands:
- `get key`
- `put key value`
- `append key value`
- `putfile key path` — reads file, base64-encodes, stores at key.
- `getfile key path` — reads base64 from key, writes decoded file to path.

Global flag:
- `-servers` (csv): Comma-separated list of server addresses (same addresses as server `-peers`).

---

## Using prebuilt binaries

Download the archive for your OS/arch and unzip:

- Windows: `kvserver.exe`, `kvclient.exe`
- Linux: `kvserver`, `kvclient` (chmod +x if needed)
- macOS: `kvserver`, `kvclient` (Apple Silicon and Intel builds)

Run the same commands shown in Quick start for your OS. No Go toolchain required.

---

## Notes and limits

- Values are plain strings. `putfile/getfile` base64-encode file contents client-side; suitable for small/medium files. Very large files increase Raft log size and disk use.
- For large files, consider chunking client-side (key: `file:name:00001`, `file:name:00002`, and a `file:name:meta`).
- Open firewall ports where servers run.
- The CLI uses timeouts and retries, automatically discovering the leader.

---

## Cross-compiling (optional)

From any OS with Go installed:

```powershell
# Windows PowerShell examples
# Linux x86_64
$env:GOOS="linux"; $env:GOARCH="amd64"; go build -o .\bin\kvserver ./cmd/kvserver; go build -o .\bin\kvclient ./cmd/kvclient

# macOS x86_64
$env:GOOS="darwin"; $env:GOARCH="amd64"; go build -o .\bin\kvserver-darwin ./cmd/kvserver; go build -o .\bin\kvclient-darwin ./cmd/kvclient

# macOS arm64 (Apple Silicon)
$env:GOOS="darwin"; $env:GOARCH="arm64"; go build -o .\bin\kvserver-darwin-arm64 ./cmd/kvserver; go build -o .\bin\kvclient-darwin-arm64 ./cmd/kvclient

# Reset env after cross builds
Remove-Item Env:GOOS, Env:GOARCH
```

---

## License

MIT
