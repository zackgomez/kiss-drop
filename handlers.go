package main

import (
	"encoding/json"
	"log"
	"net/http"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Handlers holds HTTP handlers and their dependencies
type Handlers struct {
	storage       *Storage
	uploads       *UploadManager
	baseURL       string
	defaultExpiry time.Duration
}

// NewHandlers creates a new Handlers instance
func NewHandlers(storage *Storage, uploads *UploadManager, baseURL string, defaultExpiry time.Duration) *Handlers {
	return &Handlers{
		storage:       storage,
		uploads:       uploads,
		baseURL:       strings.TrimSuffix(baseURL, "/"),
		defaultExpiry: defaultExpiry,
	}
}

// getClientIP extracts the client IP from the request, preferring X-Forwarded-For
func getClientIP(r *http.Request) string {
	// Check X-Forwarded-For header (may contain comma-separated list)
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		// Take the first IP in the list (original client)
		if idx := strings.Index(xff, ","); idx != -1 {
			return strings.TrimSpace(xff[:idx])
		}
		return strings.TrimSpace(xff)
	}
	// Check X-Real-IP header
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return xri
	}
	// Fall back to RemoteAddr (strip port)
	ip := r.RemoteAddr
	if idx := strings.LastIndex(ip, ":"); idx != -1 {
		return ip[:idx]
	}
	return ip
}

// sanitizeFileName cleans up a filename for safe storage
func sanitizeFileName(name string) string {
	// Remove path components
	name = filepath.Base(name)

	// Replace problematic characters
	reg := regexp.MustCompile(`[^a-zA-Z0-9._-]`)
	name = reg.ReplaceAllString(name, "_")

	// Limit length
	if len(name) > 200 {
		ext := filepath.Ext(name)
		name = name[:200-len(ext)] + ext
	}

	if name == "" || name == "." || name == ".." {
		name = "file"
	}

	return name
}

// HandleUpload handles POST /api/upload
func (h *Handlers) HandleUpload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse multipart form (max 10GB)
	if err := r.ParseMultipartForm(10 << 30); err != nil {
		log.Printf("Error parsing form: %v", err)
		http.Error(w, "Error parsing upload", http.StatusBadRequest)
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		log.Printf("Error getting file: %v", err)
		http.Error(w, "No file provided", http.StatusBadRequest)
		return
	}
	defer file.Close()

	fileName := sanitizeFileName(header.Filename)

	// Handle expiration
	var expiresAt *time.Time
	expiresInStr := r.FormValue("expires_in")
	if expiresInStr == "" || expiresInStr == "default" {
		// Use default expiry
		if h.defaultExpiry > 0 {
			t := time.Now().Add(h.defaultExpiry)
			expiresAt = &t
		}
	} else if expiresInStr != "never" {
		// Parse as days
		days, err := strconv.Atoi(expiresInStr)
		if err == nil && days > 0 {
			t := time.Now().Add(time.Duration(days) * 24 * time.Hour)
			expiresAt = &t
		}
	}
	// "never" means no expiration (expiresAt stays nil)

	// Capture upload metadata
	info := &UploadInfo{
		UploaderIP:  getClientIP(r),
		UserAgent:   r.UserAgent(),
		ContentType: header.Header.Get("Content-Type"),
	}

	// Create the share
	meta, err := h.storage.CreateShare(file, fileName, header.Size, expiresAt, info)
	if err != nil {
		log.Printf("Error creating share: %v", err)
		http.Error(w, "Error saving file", http.StatusInternalServerError)
		return
	}

	// Return response
	response := map[string]string{
		"id":  meta.ID,
		"url": h.baseURL + "/s/" + meta.ID,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// ShareInfoResponse is the JSON response for share metadata
type ShareInfoResponse struct {
	ID        string  `json:"id"`
	FileName  string  `json:"fileName"`
	FileSize  int64   `json:"fileSize"`
	ExpiresAt *string `json:"expiresAt,omitempty"`
}

// HandleShareInfo handles GET /api/share/:id
func (h *Handlers) HandleShareInfo(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	id := strings.TrimPrefix(r.URL.Path, "/api/share/")
	if id == "" || strings.Contains(id, "/") {
		http.Error(w, "Invalid share ID", http.StatusBadRequest)
		return
	}

	meta, err := h.storage.GetShare(id)
	if err != nil {
		log.Printf("Error getting share: %v", err)
		http.Error(w, "Internal error", http.StatusInternalServerError)
		return
	}
	if meta == nil {
		http.Error(w, "Share not found", http.StatusNotFound)
		return
	}

	response := ShareInfoResponse{
		ID:       meta.ID,
		FileName: meta.FileName,
		FileSize: meta.FileSize,
	}
	if meta.ExpiresAt != nil {
		exp := meta.ExpiresAt.Format("2006-01-02T15:04:05Z")
		response.ExpiresAt = &exp
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// HandleDownload handles GET /api/share/:id/download
func (h *Handlers) HandleDownload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract ID from path like /api/share/abc123/download
	path := strings.TrimPrefix(r.URL.Path, "/api/share/")
	path = strings.TrimSuffix(path, "/download")
	id := path

	if id == "" || strings.Contains(id, "/") {
		http.Error(w, "Invalid share ID", http.StatusBadRequest)
		return
	}

	meta, err := h.storage.GetShare(id)
	if err != nil {
		log.Printf("Error getting share: %v", err)
		http.Error(w, "Internal error", http.StatusInternalServerError)
		return
	}
	if meta == nil {
		http.Error(w, "Share not found", http.StatusNotFound)
		return
	}

	filePath := h.storage.GetFilePath(id, meta.FileName)

	// Set headers for download
	w.Header().Set("Content-Disposition", "attachment; filename=\""+meta.FileName+"\"")
	w.Header().Set("Content-Type", "application/octet-stream")

	http.ServeFile(w, r, filePath)
}

// HandleUploadInit handles POST /api/upload/init
func (h *Handlers) HandleUploadInit(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		FileName    string `json:"fileName"`
		FileSize    int64  `json:"fileSize"`
		ExpiresIn   string `json:"expiresIn,omitempty"`
		ContentType string `json:"contentType,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.FileName == "" || req.FileSize <= 0 {
		http.Error(w, "fileName and fileSize are required", http.StatusBadRequest)
		return
	}

	fileName := sanitizeFileName(req.FileName)

	// Capture upload metadata
	info := &UploadInfo{
		UploaderIP:  getClientIP(r),
		UserAgent:   r.UserAgent(),
		ContentType: req.ContentType,
	}

	session, err := h.uploads.InitUpload(fileName, req.FileSize, req.ExpiresIn, info)
	if err != nil {
		log.Printf("Error initializing upload: %v", err)
		http.Error(w, "Error initializing upload", http.StatusInternalServerError)
		return
	}

	response := InitUploadResponse{
		UploadID:    session.ID,
		ChunkSize:   session.ChunkSize,
		TotalChunks: session.TotalChunks,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// HandleUploadChunk handles POST /api/upload/:uploadId/chunk/:index
func (h *Handlers) HandleUploadChunk(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract uploadId and index from path
	// Path format: /api/upload/{uploadId}/chunk/{index}
	path := strings.TrimPrefix(r.URL.Path, "/api/upload/")
	parts := strings.Split(path, "/chunk/")
	if len(parts) != 2 {
		http.Error(w, "Invalid path", http.StatusBadRequest)
		return
	}

	uploadID := parts[0]
	index, err := strconv.Atoi(parts[1])
	if err != nil {
		http.Error(w, "Invalid chunk index", http.StatusBadRequest)
		return
	}

	session := h.uploads.GetSession(uploadID)
	if session == nil {
		http.Error(w, "Upload session not found", http.StatusNotFound)
		return
	}

	if err := h.uploads.ReceiveChunk(uploadID, index, r.Body); err != nil {
		log.Printf("Error receiving chunk: %v", err)
		http.Error(w, "Error receiving chunk", http.StatusInternalServerError)
		return
	}

	response := ChunkResponse{
		Received: h.uploads.ReceivedCount(uploadID),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// HandleUploadComplete handles POST /api/upload/:uploadId/complete
func (h *Handlers) HandleUploadComplete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract uploadId from path
	path := strings.TrimPrefix(r.URL.Path, "/api/upload/")
	uploadID := strings.TrimSuffix(path, "/complete")

	session := h.uploads.GetSession(uploadID)
	if session == nil {
		http.Error(w, "Upload session not found", http.StatusNotFound)
		return
	}

	if !h.uploads.IsComplete(uploadID) {
		http.Error(w, "Upload not complete", http.StatusBadRequest)
		return
	}

	// Calculate expiration based on session settings
	var expiresAt *time.Time
	if session.ExpiresIn == "" || session.ExpiresIn == "default" {
		if h.defaultExpiry > 0 {
			t := time.Now().Add(h.defaultExpiry)
			expiresAt = &t
		}
	} else if session.ExpiresIn != "never" {
		days, err := strconv.Atoi(session.ExpiresIn)
		if err == nil && days > 0 {
			t := time.Now().Add(time.Duration(days) * 24 * time.Hour)
			expiresAt = &t
		}
	}

	// Create the share by reading all chunks
	reader := &chunkReader{
		um:       h.uploads,
		uploadID: uploadID,
		session:  session,
	}

	// Use upload info from session
	info := &UploadInfo{
		UploaderIP:  session.UploaderIP,
		UserAgent:   session.UserAgent,
		ContentType: session.ContentType,
	}

	meta, err := h.storage.CreateShare(reader, session.FileName, session.FileSize, expiresAt, info)
	if err != nil {
		log.Printf("Error creating share: %v", err)
		http.Error(w, "Error creating share", http.StatusInternalServerError)
		return
	}

	// Cleanup the upload session
	h.uploads.Cleanup(uploadID)

	response := map[string]string{
		"id":  meta.ID,
		"url": h.baseURL + "/s/" + meta.ID,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// ShareListItem is the JSON response for a share in the list
type ShareListItem struct {
	ID          string  `json:"id"`
	FileName    string  `json:"fileName"`
	FileSize    int64   `json:"fileSize"`
	CreatedAt   string  `json:"createdAt"`
	ExpiresAt   *string `json:"expiresAt,omitempty"`
	UploaderIP  string  `json:"uploaderIP,omitempty"`
	UserAgent   string  `json:"userAgent,omitempty"`
	ContentType string  `json:"contentType,omitempty"`
}

// HandleListShares handles GET /api/shares
func (h *Handlers) HandleListShares(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse optional limit parameter
	limit := 0
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if n, err := strconv.Atoi(limitStr); err == nil && n > 0 {
			limit = n
		}
	}

	shares, err := h.storage.ListShares(limit)
	if err != nil {
		log.Printf("Error listing shares: %v", err)
		http.Error(w, "Internal error", http.StatusInternalServerError)
		return
	}

	// Convert to response format
	items := make([]ShareListItem, 0, len(shares))
	for _, meta := range shares {
		item := ShareListItem{
			ID:          meta.ID,
			FileName:    meta.FileName,
			FileSize:    meta.FileSize,
			CreatedAt:   meta.CreatedAt.Format("2006-01-02T15:04:05Z"),
			UploaderIP:  meta.UploaderIP,
			UserAgent:   meta.UserAgent,
			ContentType: meta.ContentType,
		}
		if meta.ExpiresAt != nil {
			exp := meta.ExpiresAt.Format("2006-01-02T15:04:05Z")
			item.ExpiresAt = &exp
		}
		items = append(items, item)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(items)
}
