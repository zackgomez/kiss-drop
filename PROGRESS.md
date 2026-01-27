# kiss-drop Implementation Progress

## Overview

Building a minimal self-hosted file sharing service in Go, following `DESIGN.md`.

## Phase Status

| Phase | Description | Status |
|-------|-------------|--------|
| 1 | Project Setup | Complete |
| 2 | Core Upload (Non-Resumable) | Complete |
| 3 | Download Flow | Complete |
| 4 | Password Protection | Complete |
| 5 | Basic UI | Complete |
| 6 | Expiration | Complete |
| 7 | Resumable Uploads | Not started |
| 8 | Docker | Not started |

---

## Phase 1: Project Setup

**Goal:** Initialize Go module, basic HTTP server responding on `/`

### Plan
- `go mod init github.com/zackgomez/kiss-drop`
- Create `main.go` with simple HTTP server
- Test with `curl http://localhost:8080/`

### Implementation Notes
- Initialized module as `github.com/zackgomez/kiss-drop`
- Created minimal `main.go` with HTTP server on configurable PORT (default 8080)
- Server responds with "kiss-drop is running" on `/`

### Testing
```bash
curl http://localhost:8080/
# Returns: kiss-drop is running
```

---

## Phase 2: Core Upload (Non-Resumable)

**Goal:** Basic single-file upload via multipart form

### Plan
- Create `storage.go` for file operations
- Implement `POST /api/upload` handler
- Generate 8-char nanoid for share ID
- Save file + `meta.json` to `data/shares/{id}/`
- Return `{ id, url }`

### Implementation Notes
- Created `storage.go` with `Storage` struct for file operations
- `GenerateID()` creates 8-char base62 IDs using crypto/rand
- `CreateShare()` saves file + meta.json to `data/shares/{id}/`
- Created `handlers.go` with `HandleUpload` for POST /api/upload
- `sanitizeFileName()` cleans filenames (removes path components, limits chars)
- Returns JSON with `id` and `url`

### Testing
```bash
curl -X POST -F "file=@test.txt" http://localhost:8080/api/upload
# Returns: {"id":"LxhXurlH","url":"http://localhost:8080/s/LxhXurlH"}
```

---

## Phase 3: Download Flow

**Goal:** Serve files and metadata

### Plan
- Implement `GET /api/share/:id` - return metadata JSON
- Implement `GET /api/share/:id/download` - serve file with Content-Disposition: attachment

### Implementation Notes
- Added `HandleShareInfo` for GET /api/share/:id - returns JSON with file metadata
- Added `HandleDownload` for GET /api/share/:id/download - serves file with Content-Disposition: attachment
- Used path prefix routing in main.go since Go's default mux doesn't support path params

### Testing
```bash
# Get share info
curl http://localhost:8080/api/share/1itfPbHF
# Returns: {"id":"1itfPbHF","fileName":"test.txt","fileSize":18,"passwordRequired":false}

# Download file
curl http://localhost:8080/api/share/1itfPbHF/download
# Returns: file contents
```

---

## Phase 4: Password Protection

**Goal:** Optional password protection on shares

### Plan
- Add optional `password` field to upload
- Hash with argon2id before storing
- Implement `POST /api/share/:id/unlock` - verify password, set signed cookie
- Gate download behind cookie check

### Implementation Notes
- Created `auth.go` with argon2id password hashing (using golang.org/x/crypto/argon2)
- `HashPassword()` generates salt + hash, encodes as `salt:hash` in hex
- `VerifyPassword()` uses constant-time comparison
- Cookie-based unlock with HMAC-signed cookies (24h expiry)
- `COOKIE_SECRET` env var for persistent sessions across restarts
- Download endpoint checks for valid unlock cookie before serving

### Testing
```bash
# Upload with password
curl -X POST -F "file=@test.txt" -F "password=secret123" http://localhost:8080/api/upload

# Shows passwordRequired: true
curl http://localhost:8080/api/share/$ID
# Returns: {"passwordRequired":true,...}

# Download blocked
curl http://localhost:8080/api/share/$ID/download
# Returns: 401 Password required

# Unlock
curl -c cookies.txt -X POST -H "Content-Type: application/json" \
  -d '{"password":"secret123"}' http://localhost:8080/api/share/$ID/unlock

# Download with cookie works
curl -b cookies.txt http://localhost:8080/api/share/$ID/download
```

---

## Phase 5: Basic UI

**Goal:** HTML templates for upload and download pages

### Plan
- Create `templates/upload.html` - drag-and-drop upload form
- Create `templates/download.html` - file info + password prompt if needed
- Add `static/style.css` - minimal styling
- Serve UI at `/` (upload) and `/s/:id` (download)

### Implementation Notes
- Created `templates.go` with embed directives for templates and static files
- `templates/upload.html` - drag-and-drop upload with progress bar, password option
- `templates/download.html` - file info, password unlock form, download button
- `static/style.css` - clean minimal styling
- All assets embedded in binary using Go's `//go:embed`
- Upload page uses XHR for progress tracking
- Download page conditionally shows unlock form based on passwordRequired

### Testing
- Open http://localhost:8080/ in browser
- Drag and drop a file, optionally set password
- After upload, copy the share link
- Visit the share link to see download page
- Password-protected shares show unlock form

---

## Phase 6: Expiration

**Goal:** Auto-cleanup of expired shares

### Plan
- Add `expires_at` to metadata (configurable, default 30d)
- Background goroutine scans hourly, deletes expired
- Log deletions

### Implementation Notes
- Added `expires_in` form field to upload (accepts days or "never")
- `DEFAULT_EXPIRY` env var (default 30d) sets default expiration
- Added expiration dropdown to upload UI (1d, 7d, 30d, 90d, never)
- `CleanupExpired()` in storage.go scans and deletes expired shares
- Background goroutine runs cleanup every hour
- Runs immediately on startup, then hourly

### Testing
```bash
# Upload with 1 day expiry
curl -X POST -F "file=@test.txt" -F "expires_in=1" http://localhost:8080/api/upload

# Check metadata shows expiresAt
curl http://localhost:8080/api/share/$ID
# Returns: {"expiresAt":"2026-01-27T21:32:39Z",...}

# Upload with never expiry
curl -X POST -F "file=@test.txt" -F "expires_in=never" http://localhost:8080/api/upload
# No expiresAt in response
```

---

## Phase 7: Resumable Uploads

**Goal:** Chunked uploads for large files

### Plan
- Implement `/api/upload/init` - start upload session
- Implement `/api/upload/:uploadId/chunk/:index` - receive chunks
- Implement `/api/upload/:uploadId/complete` - assemble file
- Create `static/upload.js` for client-side chunking

### Implementation Notes
_To be filled in during implementation_

### Testing
_To be filled in during implementation_

---

## Phase 8: Docker

**Goal:** Production-ready container

### Plan
- Multi-stage Dockerfile (build + scratch/alpine)
- Target < 50MB image size
- Test build and run

### Implementation Notes
_To be filled in during implementation_

### Testing
_To be filled in during implementation_

---

## Decisions Log

| Date | Decision | Rationale |
|------|----------|-----------|
| | | |

## Issues Encountered

| Phase | Issue | Resolution |
|-------|-------|------------|
| | | |
