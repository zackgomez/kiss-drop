# kiss-drop Implementation Progress

## Overview

Building a minimal self-hosted file sharing service in Go, following `DESIGN.md`.

## Phase Status

| Phase | Description | Status |
|-------|-------------|--------|
| 1 | Project Setup | Complete |
| 2 | Core Upload (Non-Resumable) | Complete |
| 3 | Download Flow | Not started |
| 4 | Password Protection | Not started |
| 5 | Basic UI | Not started |
| 6 | Expiration | Not started |
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
_To be filled in during implementation_

### Testing
_To be filled in during implementation_

---

## Phase 4: Password Protection

**Goal:** Optional password protection on shares

### Plan
- Add optional `password` field to upload
- Hash with argon2id before storing
- Implement `POST /api/share/:id/unlock` - verify password, set signed cookie
- Gate download behind cookie check

### Implementation Notes
_To be filled in during implementation_

### Testing
_To be filled in during implementation_

---

## Phase 5: Basic UI

**Goal:** HTML templates for upload and download pages

### Plan
- Create `templates/upload.html` - drag-and-drop upload form
- Create `templates/download.html` - file info + password prompt if needed
- Add `static/style.css` - minimal styling
- Serve UI at `/` (upload) and `/s/:id` (download)

### Implementation Notes
_To be filled in during implementation_

### Testing
_To be filled in during implementation_

---

## Phase 6: Expiration

**Goal:** Auto-cleanup of expired shares

### Plan
- Add `expires_at` to metadata (configurable, default 30d)
- Background goroutine scans hourly, deletes expired
- Log deletions

### Implementation Notes
_To be filled in during implementation_

### Testing
_To be filled in during implementation_

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
