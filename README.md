# BitTorrent-Lyte

A BitTorrent-style peer-to-peer file-sharing system written in Go. Peers find each other through an HTTP tracker, then transfer files directly over a custom TCP protocol. Every piece is verified with SHA-256 before it is accepted.

**Highlights**

- **2.1x faster downloads** with parallel workers (332 to 698 MB/s, 100 MB file, localhost)
- **Fault tolerant:** failed pieces are automatically retried from alternate peers
- **Tracker handles 20,000 requests/sec** at 100 concurrent clients with zero failures and zero data races
- **59 automated tests**, all passing under Go's race detector
- **Standard library only:** no third-party dependencies

---

## How it works

```
<img width="892" height="591" alt="how it works" src="https://github.com/user-attachments/assets/dd297cec-590b-4ee2-b79b-31e423ea3c0d" />

```

1. **Seed:** a peer splits a file into pieces, hashes each one, and writes a `.torrent` metadata file. It registers with the tracker.
2. **Discover:** a downloader loads the `.torrent` file and asks the tracker for peers holding the file.
3. **Download:** worker goroutines fetch pieces in parallel over TCP. Each piece is requested as 16 KiB blocks, with up to 16 requests in flight at once.
4. **Verify and reassemble:** each piece is checked against its SHA-256 hash, then written to the output file. The finished peer becomes a seed itself.

### Download engine

```
 <img width="1395" height="734" alt="DOWNLOAD ENGINE" src="https://github.com/user-attachments/assets/82365c24-cfd4-46b1-9ede-98fcea5b37c4" />

```

---

## Quick start

Requires Go 1.27.1

```bash
# Terminal 1: start the tracker
go run ./tracker

# Terminal 2: seed a file (creates movie.torrent)
go run . -mode seed -file movie.mp4 -torrent movie.torrent \
  -port 6881 -tracker http://127.0.0.1:8080/announce

# Terminal 3: download it
go run . -mode download -torrent movie.torrent -output copy.mp4 \
  -port 6882 -tracker http://127.0.0.1:8080/announce -workers 5
```

The downloader keeps running after it finishes and seeds the file to others. Press `Ctrl+C` to stop.

### Options

| Flag | Default | Description |
|---|---|---|
| `-mode` | | `seed` or `download` |
| `-file` | | File to seed (seed mode) |
| `-torrent` | | Path of the `.torrent` metadata file |
| `-output` | | Where to save the file (download mode) |
| `-tracker` | `http://localhost:8080/announce` | Tracker URL |
| `-port` | `6881` | Port to listen on for other peers |
| `-workers` | `5` | Parallel download workers |
| `-piece-size` | `262144` | Piece size in bytes (seed mode) |

---

## Benchmarks

Measured on a MacBook Pro (M1 PRO), everything running on localhost. Downloading a 100 MB file (400 pieces) from one seeder. Output was byte-identical to the original in every run.

| Workers | Time | Throughput |
|---|---|---|
| 1 | 0.30 s | 332 MB/s |
| 5 | 0.14 s | **698 MB/s** |
| 10 | 0.17 s | 577 MB/s |

Throughput peaks at 5 workers. Beyond that, one seeder and a fresh TCP connection per piece become the bottleneck.

**Tracker load test** (ApacheBench, 20,000 requests, 100 concurrent, run with the race detector on): 20,084 requests/sec, 0 failed requests, 99% of requests under 9 ms, 0 data races.

> Localhost numbers show the software's overhead, not real network speed.

---

## Design notes

**Concurrency.** The tracker stores swarms and peers in maps protected by `sync.RWMutex`, so many peers can read the peer list at once while writes stay safe. Downloads use a worker pool: a buffered job queue feeds N goroutines, and results flow back through a channel.

**Integrity.** The metadata stores a SHA-256 hash per piece. A downloaded piece that fails its hash check is discarded and retried from a different peer. The torrent's identity (info hash) is the SHA-256 of its metadata.

**Protocol hardening.** Peers are untrusted, so the client and upload server reject:

- messages larger than 1 MiB
- blocks that are misaligned, wrongly sized, duplicated, or never requested
- requests for pieces the peer does not have
- stalled connections (30 s read and write deadlines)

**Wire protocol.** An 80-byte handshake (protocol string, info hash, peer ID), followed by length-prefixed `request` and `piece` messages.

---

## Project structure

```
.
├── main.go        CLI: seed and download modes
├── tracker/       HTTP tracker (swarms, peers, announce handler)
├── client/        Tracker announce, download orchestration, upload server
├── peer/          Handshake, wire messages, worker pool, block pipelining
├── chunker/       File splitting, SHA-256 hashing, .torrent metadata
├── bitfield/      Thread-safe piece-availability bitfield
├── integrity/     SHA-256 piece verification
└── storage/       File reassembly
```

## Testing

```bash
go test ./... -race -cover
```

59 tests across all packages, including tracker validation, wire-protocol edge cases, and end-to-end transfers.

## Limitations

This is a learning project for concurrency and distributed systems, intentionally simplified compared to the real BitTorrent protocol.

- Uses JSON metadata and SHA-256 instead of bencode and SHA-1, so it is **not** compatible with real BitTorrent clients
- The tracker only returns seeds, so a peer is discoverable only after it finishes downloading
- Pieces are assigned round-robin, not rarest-first
- A new TCP connection is opened for each piece. Connection reuse would raise throughput.
- No NAT traversal, DHT, or encryption

## Gen AI Acknowledgment

Core logic was written by hand over the course of Early October - November 2026. Most unit tests were AI generated with human oversight, with A.I also assisting on making the project README free from grammar errors and concise.
