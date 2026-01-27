# Simple File Share - Design Document

## Overview

A minimal self-hosted file sharing service. Upload a file via web UI, get a shareable link, optionally protect with a password.

## Goals

1. **Simple to deploy** - Single Docker container, no external dependencies
2. **Simple to use** - Drop file, set options, get link
3. **Small codebase** - Easy to understand and modify

## Non-Goals

- User accounts or authentication
- Admin UI (configure via environment variables)
- Multiple storage backends (local filesystem only)
- Email notifications
- File preview
- Multi-file shares or zip downloads
- Analytics or download tracking
- Mobile app

## Requirements

### Functional

| ID | Requirement |
|----|-------------|
| F1 | Upload single file via drag-and-drop or file picker |
| F2 | Resumable uploads for large files |
| F3 | Configure expiration (default 30 days, permanent allowed) |
| F4 | Optionally set password when uploading |
| F5 | Generate unique shareable link |
| F6 | Download page shows file metadata (name, size) |
| F7 | Password-protected shares prompt before allowing download |

### Non-Functional

| ID | Requirement |
|----|-------------|
| N1 | Single Docker image, < 50MB |
| N2 | No runtime dependencies |
| N3 | Configurable via environment variables |
| N4 | Handles files up to 10GB |
| N5 | Works behind reverse proxy (X-Forwarded headers) |

## Data Model

```
Share
├── id: string (nanoid, 8 chars)
├── created_at: timestamp
├── expires_at: timestamp? (null = permanent)
├── password_hash: string? (argon2, null = no password)
├── file_name: string (original filename)
├── file_size: int64 (bytes)
```

Storage: flat files (SQLite in v2 if needed)
```
/data
└── shares
    └── abc123xy
        ├── meta.json
        └── document.pdf
```

## API Design

```
POST /api/upload
  - Chunked/resumable upload (tus protocol or custom)
  - Form fields: password?, expires_in?
  - Returns: { id, url }

GET /api/share/:id
  - Returns: { id, fileName, fileSize, expiresAt, passwordRequired: bool }

POST /api/share/:id/unlock
  - Body: { password }
  - Sets signed cookie, returns 200 or 401

GET /api/share/:id/download
  - Returns: file stream (or 401 if locked)
```

Password unlock flow:
- Successful unlock sets signed cookie scoped to share ID
- Cookie expires in 24h

## UI Design

**Upload Page (`/`)**
```
┌─────────────────────────────────────┐
│                                     │
│     Drop file here or click to      │
│            select file              │
│                                     │
└─────────────────────────────────────┘

  Expires: [30 days ▼]  Password: [________]

            [ Upload ]

---after upload---

  Share link: https://share.example.com/s/abc123xy  [Copy]
```

**Download Page (`/s/:id`)**
```
  document.pdf
  4.2 MB · Expires Jan 30, 2026

  Password: [________]  [Unlock]

---after unlock (or if no password)---

  document.pdf
  4.2 MB · Expires Jan 30, 2026

            [ Download ]
```

Frontend: Go html/template + vanilla JS (no build step)

## Resumable Uploads

Options:
- **tus protocol** - Standard, client libraries exist, but adds complexity
- **Custom chunked** - Simple: split client-side, reassemble server-side

Leaning toward custom chunked for learning value:
```
POST /api/upload/init
  - Body: { fileName, fileSize, password?, expiresIn? }
  - Returns: { uploadId, chunkSize }

POST /api/upload/:uploadId/chunk/:index
  - Body: raw chunk bytes
  - Returns: { received: int }

POST /api/upload/:uploadId/complete
  - Validates all chunks received, assembles file
  - Returns: { id, url }
```

## Configuration

| Env Var | Default | Description |
|---------|---------|-------------|
| `PORT` | 8080 | HTTP port |
| `DATA_DIR` | /data | Storage directory |
| `MAX_FILE_SIZE` | 10GB | Maximum file size |
| `BASE_URL` | http://localhost:8080 | For generating links |
| `DEFAULT_EXPIRY` | 30d | Default expiration |
| `MAX_EXPIRY` | 0 | Max expiration (0 = unlimited) |

## Security Considerations

- Passwords hashed with argon2id
- Share IDs are random (8 chars base62 = ~48 bits entropy)
- Sanitize filenames on upload (strip path components, limit chars)
- Set `Content-Disposition: attachment` to prevent XSS
- Rate limit password attempts (v2)

## Project Structure

```
simple-file-share/
├── main.go           # Entry, config, server setup
├── handlers.go       # HTTP handlers
├── storage.go        # File + metadata operations
├── upload.go         # Chunked upload handling
├── templates/
│   ├── upload.html
│   └── download.html
├── static/
│   ├── style.css
│   └── upload.js     # Chunked upload client
├── Dockerfile
└── README.md
```

Target: ~1000 lines of Go

## Expiration Cleanup

Background goroutine runs periodically (hourly):
- Scans share directories
- Deletes shares past expiration
- Logs deletions

## Future (v2)

- SQLite for metadata (enables search, stats)
- Rate limiting on password attempts
- Optional virus scanning
- Bandwidth throttling

## References

- Analyzed Pingvin Share (~31k lines TS) as prior art
