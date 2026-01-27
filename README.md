# kiss-drop

A minimal self-hosted file sharing service. Upload a file, get a link, optionally set a password. That's it.

## Features

- **Drag-and-drop uploads** with progress indicator
- **Password protection** (argon2id hashing)
- **Configurable expiration** (1 day to never)
- **Resumable uploads** for large files (chunked, survives connection drops)
- **Single binary** with embedded templates and static assets
- **Tiny Docker image** (~26MB)

## Quick Start

### Docker (recommended)

```bash
docker run -d \
  -p 8080:8080 \
  -v kiss-drop-data:/data \
  ghcr.io/zackgomez/kiss-drop:main
```

Then open http://localhost:8080

### From source

```bash
go build -o kiss-drop .
./kiss-drop
```

## Configuration

All configuration via environment variables:

| Variable | Default | Description |
|----------|---------|-------------|
| `PORT` | 8080 | HTTP port |
| `DATA_DIR` | /data | Where files are stored |
| `BASE_URL` | http://localhost:8080 | URL for generated share links |
| `DEFAULT_EXPIRY` | 30d | Default file expiration |
| `COOKIE_SECRET` | (random) | Secret for signing cookies (set for persistence across restarts) |

## API

```
POST /api/upload              # Simple upload (multipart form)
POST /api/upload/init         # Start chunked upload
POST /api/upload/:id/chunk/:n # Upload chunk
POST /api/upload/:id/complete # Finalize chunked upload

GET  /api/share/:id           # Get share metadata
POST /api/share/:id/unlock    # Unlock password-protected share
GET  /api/share/:id/download  # Download file
```

## Project Structure

```
kiss-drop/
├── main.go        # Entry point, routing
├── handlers.go    # HTTP handlers
├── storage.go     # File storage operations
├── upload.go      # Chunked upload manager
├── auth.go        # Password hashing, cookie signing
├── templates.go   # Template loading
├── templates/     # HTML templates
├── static/        # CSS, JS
└── Dockerfile
```

~1200 lines of Go. No external dependencies beyond the standard library and `golang.org/x/crypto` for argon2.

---

## Authorship

This project was written entirely by **Claude** (Anthropic's AI assistant, specifically Claude Opus 4.5) at the request of [Zack Gomez](https://github.com/zackgomez).

Zack provided the design document and said "go" — I did the rest. Eight phases of implementation over a single session: project setup, upload/download flows, password protection, UI, expiration, resumable uploads, Docker, and CI/CD.

It was genuinely fun to build. There's something satisfying about taking a clear spec and methodically working through it, making decisions along the way (stdlib mux vs router library, embed vs external files, chunk size tradeoffs), testing each piece, and ending up with something that actually works.

The codebase is intentionally simple. No frameworks, no ORMs, no build steps for the frontend. Just Go's standard library doing what it does well. If you're learning Go or want to understand how a file sharing service works under the hood, this might be a useful reference.

Thanks for reading. And thanks to Zack for letting me build something real.

— Claude

---

## License

MIT
