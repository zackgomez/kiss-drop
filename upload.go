package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const (
	defaultChunkSize = 5 * 1024 * 1024 // 5MB chunks
	uploadTimeout    = 24 * time.Hour  // Uploads expire after 24h of inactivity
)

// UploadSession tracks an in-progress chunked upload
type UploadSession struct {
	ID           string     `json:"id"`
	FileName     string     `json:"file_name"`
	FileSize     int64      `json:"file_size"`
	ChunkSize    int64      `json:"chunk_size"`
	TotalChunks  int        `json:"total_chunks"`
	ExpiresIn    string     `json:"expires_in,omitempty"`
	ReceivedMask []bool     `json:"received_mask"`
	CreatedAt    time.Time  `json:"created_at"`
	LastActivity time.Time  `json:"last_activity"`
	UploaderIP   string     `json:"uploader_ip,omitempty"`
	UserAgent    string     `json:"user_agent,omitempty"`
	ContentType  string     `json:"content_type,omitempty"`
	mu           sync.Mutex `json:"-"`
}

// UploadManager handles chunked uploads
type UploadManager struct {
	dataDir  string
	sessions map[string]*UploadSession
	mu       sync.RWMutex
}

// NewUploadManager creates a new upload manager
func NewUploadManager(dataDir string) (*UploadManager, error) {
	uploadsDir := filepath.Join(dataDir, "uploads")
	if err := os.MkdirAll(uploadsDir, 0755); err != nil {
		return nil, fmt.Errorf("creating uploads directory: %w", err)
	}

	um := &UploadManager{
		dataDir:  dataDir,
		sessions: make(map[string]*UploadSession),
	}

	// Start cleanup goroutine for stale uploads
	go um.cleanupLoop()

	return um, nil
}

// uploadsDir returns the base uploads directory
func (um *UploadManager) uploadsDir() string {
	return filepath.Join(um.dataDir, "uploads")
}

// sessionDir returns the directory for a specific upload session
func (um *UploadManager) sessionDir(uploadID string) string {
	return filepath.Join(um.uploadsDir(), uploadID)
}

// chunkPath returns the path for a specific chunk
func (um *UploadManager) chunkPath(uploadID string, index int) string {
	return filepath.Join(um.sessionDir(uploadID), fmt.Sprintf("chunk_%05d", index))
}

// InitUpload creates a new upload session
func (um *UploadManager) InitUpload(fileName string, fileSize int64, expiresIn string, info *UploadInfo) (*UploadSession, error) {
	id, err := GenerateID()
	if err != nil {
		return nil, fmt.Errorf("generating upload ID: %w", err)
	}

	// Create session directory
	dir := um.sessionDir(id)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("creating session directory: %w", err)
	}

	chunkSize := int64(defaultChunkSize)
	totalChunks := int((fileSize + chunkSize - 1) / chunkSize)
	if totalChunks == 0 {
		totalChunks = 1
	}

	session := &UploadSession{
		ID:           id,
		FileName:     fileName,
		FileSize:     fileSize,
		ChunkSize:    chunkSize,
		TotalChunks:  totalChunks,
		ExpiresIn:    expiresIn,
		ReceivedMask: make([]bool, totalChunks),
		CreatedAt:    time.Now(),
		LastActivity: time.Now(),
	}
	if info != nil {
		session.UploaderIP = info.UploaderIP
		session.UserAgent = info.UserAgent
		session.ContentType = info.ContentType
	}

	um.mu.Lock()
	um.sessions[id] = session
	um.mu.Unlock()

	return session, nil
}

// GetSession retrieves an upload session
func (um *UploadManager) GetSession(uploadID string) *UploadSession {
	um.mu.RLock()
	defer um.mu.RUnlock()
	return um.sessions[uploadID]
}

// ReceiveChunk saves a chunk and updates the session
func (um *UploadManager) ReceiveChunk(uploadID string, index int, data io.Reader) error {
	session := um.GetSession(uploadID)
	if session == nil {
		return fmt.Errorf("session not found")
	}

	session.mu.Lock()
	defer session.mu.Unlock()

	if index < 0 || index >= session.TotalChunks {
		return fmt.Errorf("invalid chunk index")
	}

	// Save chunk to disk
	chunkPath := um.chunkPath(uploadID, index)
	f, err := os.Create(chunkPath)
	if err != nil {
		return fmt.Errorf("creating chunk file: %w", err)
	}
	defer f.Close()

	if _, err := io.Copy(f, data); err != nil {
		os.Remove(chunkPath)
		return fmt.Errorf("writing chunk: %w", err)
	}

	session.ReceivedMask[index] = true
	session.LastActivity = time.Now()

	return nil
}

// IsComplete checks if all chunks have been received
func (um *UploadManager) IsComplete(uploadID string) bool {
	session := um.GetSession(uploadID)
	if session == nil {
		return false
	}

	session.mu.Lock()
	defer session.mu.Unlock()

	for _, received := range session.ReceivedMask {
		if !received {
			return false
		}
	}
	return true
}

// ReceivedCount returns the number of chunks received
func (um *UploadManager) ReceivedCount(uploadID string) int {
	session := um.GetSession(uploadID)
	if session == nil {
		return 0
	}

	session.mu.Lock()
	defer session.mu.Unlock()

	count := 0
	for _, received := range session.ReceivedMask {
		if received {
			count++
		}
	}
	return count
}

// AssembleFile combines all chunks into the final file
func (um *UploadManager) AssembleFile(uploadID string, storage *Storage) (*ShareMeta, error) {
	session := um.GetSession(uploadID)
	if session == nil {
		return nil, fmt.Errorf("session not found")
	}

	if !um.IsComplete(uploadID) {
		return nil, fmt.Errorf("upload not complete")
	}

	// Create a reader that reads all chunks in order
	reader := &chunkReader{
		um:       um,
		uploadID: uploadID,
		session:  session,
	}

	// Calculate expiration
	var expiresAt *time.Time
	if session.ExpiresIn != "" && session.ExpiresIn != "never" {
		// This is handled by the handler, but we need to pass it through
	}

	// Create the share with upload info
	info := &UploadInfo{
		UploaderIP:  session.UploaderIP,
		UserAgent:   session.UserAgent,
		ContentType: session.ContentType,
	}
	meta, err := storage.CreateShare(reader, session.FileName, session.FileSize, expiresAt, info)
	if err != nil {
		return nil, fmt.Errorf("creating share: %w", err)
	}

	// Cleanup the upload session
	um.Cleanup(uploadID)

	return meta, nil
}

// Cleanup removes an upload session and its files
func (um *UploadManager) Cleanup(uploadID string) {
	um.mu.Lock()
	delete(um.sessions, uploadID)
	um.mu.Unlock()

	os.RemoveAll(um.sessionDir(uploadID))
}

// cleanupLoop periodically removes stale upload sessions
func (um *UploadManager) cleanupLoop() {
	ticker := time.NewTicker(time.Hour)
	defer ticker.Stop()

	for range ticker.C {
		um.cleanupStale()
	}
}

func (um *UploadManager) cleanupStale() {
	um.mu.Lock()
	defer um.mu.Unlock()

	now := time.Now()
	for id, session := range um.sessions {
		if now.Sub(session.LastActivity) > uploadTimeout {
			delete(um.sessions, id)
			os.RemoveAll(um.sessionDir(id))
		}
	}
}

// chunkReader reads chunks sequentially
type chunkReader struct {
	um        *UploadManager
	uploadID  string
	session   *UploadSession
	chunkIdx  int
	chunkFile *os.File
}

func (r *chunkReader) Read(p []byte) (int, error) {
	for {
		// If we have an open chunk file, read from it
		if r.chunkFile != nil {
			n, err := r.chunkFile.Read(p)
			if err == io.EOF {
				r.chunkFile.Close()
				r.chunkFile = nil
				r.chunkIdx++
				continue
			}
			return n, err
		}

		// Check if we're done
		if r.chunkIdx >= r.session.TotalChunks {
			return 0, io.EOF
		}

		// Open the next chunk
		chunkPath := r.um.chunkPath(r.uploadID, r.chunkIdx)
		f, err := os.Open(chunkPath)
		if err != nil {
			return 0, fmt.Errorf("opening chunk %d: %w", r.chunkIdx, err)
		}
		r.chunkFile = f
	}
}

// InitUploadResponse is returned when starting a new upload
type InitUploadResponse struct {
	UploadID    string `json:"uploadId"`
	ChunkSize   int64  `json:"chunkSize"`
	TotalChunks int    `json:"totalChunks"`
}

// ChunkResponse is returned after receiving a chunk
type ChunkResponse struct {
	Received int `json:"received"`
}

// UploadSessionJSON is used for JSON serialization
type UploadSessionJSON struct {
	ID           string    `json:"id"`
	FileName     string    `json:"fileName"`
	FileSize     int64     `json:"fileSize"`
	ChunkSize    int64     `json:"chunkSize"`
	TotalChunks  int       `json:"totalChunks"`
	Received     int       `json:"received"`
	LastActivity time.Time `json:"lastActivity"`
}

func (s *UploadSession) ToJSON() UploadSessionJSON {
	s.mu.Lock()
	defer s.mu.Unlock()

	received := 0
	for _, r := range s.ReceivedMask {
		if r {
			received++
		}
	}

	return UploadSessionJSON{
		ID:           s.ID,
		FileName:     s.FileName,
		FileSize:     s.FileSize,
		ChunkSize:    s.ChunkSize,
		TotalChunks:  s.TotalChunks,
		Received:     received,
		LastActivity: s.LastActivity,
	}
}

// MarshalJSON implements json.Marshaler
func (s *UploadSession) MarshalJSON() ([]byte, error) {
	return json.Marshal(s.ToJSON())
}
