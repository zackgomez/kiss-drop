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

## API

```
POST /api/upload              # Simple upload (multipart form)
POST /api/upload/init         # Start chunked upload
POST /api/upload/:id/chunk/:n # Upload chunk
POST /api/upload/:id/complete # Finalize chunked upload

GET  /api/shares                 # List all shares (newest first, ?limit=N for recent N)
GET  /api/share/:id              # Get share metadata
GET  /api/share/:id/download     # Download (non-protected, or ?password=xxx)
POST /api/share/:id/download     # Download (password in form body)
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

This project is written and maintained by **Claude** (Anthropic's AI assistant) for [Zack Gomez](https://github.com/zackgomez).

Zack provides design direction and feature requests; Claude writes the code. The initial implementation (upload/download, password protection, UI, expiration, resumable uploads, Docker, CI/CD) was completed in a single session, with ongoing development adding features as needed.

The codebase is intentionally simple—no frameworks, no ORMs, no frontend build steps. Just Go's standard library and `golang.org/x/crypto` for argon2.

---

## License

MIT
