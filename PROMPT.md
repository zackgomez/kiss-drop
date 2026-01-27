# kiss-drop Development Prompt

You are starting work on **kiss-drop**, a minimal self-hosted file sharing service written in Go.

## Context

- Design doc: `DESIGN.md` in this repo - read it first
- This is a learning project (Go is new) so write idiomatic, readable code
- GitHub repo: https://github.com/zackgomez/kiss-drop (private)

## Tools Available

- Go 1.25.6 via mise (run `eval "$(/home/zack/.local/bin/mise activate bash)"` before go commands)
- Docker
- Standard Linux tools

## Your Task

Implement kiss-drop following the design doc. Work incrementally:

### Phase 1: Project Setup
1. Initialize Go module
2. Set up basic project structure (main.go, go.mod)
3. Create a simple HTTP server that responds on `/`
4. Test it runs

### Phase 2: Core Upload (Non-Resumable First)
1. Implement basic single-file upload (multipart form, no chunking yet)
2. Generate share ID, save file to disk with meta.json
3. Return share URL
4. Test with curl

### Phase 3: Download Flow
1. Implement GET /api/share/:id (return metadata)
2. Implement GET /api/share/:id/download (serve file)
3. Test with curl

### Phase 4: Password Protection
1. Add optional password on upload (hash with argon2 or bcrypt)
2. Implement POST /api/share/:id/unlock (set cookie)
3. Gate download behind password check
4. Test with curl

### Phase 5: Basic UI
1. Create upload page template (html/template)
2. Create download page template
3. Add minimal CSS
4. Test in browser

### Phase 6: Expiration
1. Add expires_at to metadata
2. Implement cleanup goroutine
3. Test expiration works

### Phase 7: Resumable Uploads
1. Implement chunked upload endpoints
2. Add upload.js for client-side chunking
3. Test with large file

### Phase 8: Docker
1. Create Dockerfile (multi-stage build)
2. Test docker build and run
3. Verify image size < 50MB

## Guidelines

- **Test each phase** before moving on - use curl, then browser
- **Commit after each phase** with a descriptive message
- **Keep PROGRESS.md updated** with:
  - What you implemented
  - Decisions made and why
  - Any issues encountered
  - Commands used for testing
- **Don't over-engineer** - simplest thing that works
- **System packages**: You can install standard packages (e.g., `apt install curl`) but don't modify system config or install anything unusual
- **Ask if stuck** - don't spin on issues

## Before You Start

Create `PROGRESS.md` and wait for user to say "go" before beginning implementation.
