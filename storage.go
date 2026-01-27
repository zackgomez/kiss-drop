package main

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

// ShareMeta holds metadata for a shared file
type ShareMeta struct {
	ID           string     `json:"id"`
	CreatedAt    time.Time  `json:"created_at"`
	ExpiresAt    *time.Time `json:"expires_at,omitempty"`
	PasswordHash string     `json:"password_hash,omitempty"`
	FileName     string     `json:"file_name"`
	FileSize     int64      `json:"file_size"`
}

// Storage handles file and metadata operations
type Storage struct {
	dataDir string
}

// NewStorage creates a new Storage instance
func NewStorage(dataDir string) (*Storage, error) {
	sharesDir := filepath.Join(dataDir, "shares")
	if err := os.MkdirAll(sharesDir, 0755); err != nil {
		return nil, fmt.Errorf("creating shares directory: %w", err)
	}
	return &Storage{dataDir: dataDir}, nil
}

// GenerateID creates a random 8-character ID using base62
func GenerateID() (string, error) {
	const alphabet = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
	bytes := make([]byte, 8)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	for i := range bytes {
		bytes[i] = alphabet[bytes[i]%62]
	}
	return string(bytes), nil
}

// shareDir returns the directory path for a share
func (s *Storage) shareDir(id string) string {
	return filepath.Join(s.dataDir, "shares", id)
}

// metaPath returns the path to the meta.json file for a share
func (s *Storage) metaPath(id string) string {
	return filepath.Join(s.shareDir(id), "meta.json")
}

// filePath returns the path to the uploaded file for a share
func (s *Storage) filePath(id, fileName string) string {
	return filepath.Join(s.shareDir(id), fileName)
}

// CreateShare creates a new share with the given file
func (s *Storage) CreateShare(file io.Reader, fileName string, fileSize int64, expiresAt *time.Time, passwordHash string) (*ShareMeta, error) {
	id, err := GenerateID()
	if err != nil {
		return nil, fmt.Errorf("generating ID: %w", err)
	}

	// Create share directory
	dir := s.shareDir(id)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("creating share directory: %w", err)
	}

	// Save the file
	filePath := s.filePath(id, fileName)
	dst, err := os.Create(filePath)
	if err != nil {
		os.RemoveAll(dir)
		return nil, fmt.Errorf("creating file: %w", err)
	}
	defer dst.Close()

	written, err := io.Copy(dst, file)
	if err != nil {
		os.RemoveAll(dir)
		return nil, fmt.Errorf("writing file: %w", err)
	}

	// Create metadata
	meta := &ShareMeta{
		ID:           id,
		CreatedAt:    time.Now().UTC(),
		ExpiresAt:    expiresAt,
		PasswordHash: passwordHash,
		FileName:     fileName,
		FileSize:     written,
	}

	// Save metadata
	if err := s.saveMeta(meta); err != nil {
		os.RemoveAll(dir)
		return nil, err
	}

	return meta, nil
}

// GetShare retrieves metadata for a share
func (s *Storage) GetShare(id string) (*ShareMeta, error) {
	data, err := os.ReadFile(s.metaPath(id))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading metadata: %w", err)
	}

	var meta ShareMeta
	if err := json.Unmarshal(data, &meta); err != nil {
		return nil, fmt.Errorf("parsing metadata: %w", err)
	}

	return &meta, nil
}

// GetFilePath returns the full path to a share's file
func (s *Storage) GetFilePath(id, fileName string) string {
	return s.filePath(id, fileName)
}

// saveMeta writes metadata to disk
func (s *Storage) saveMeta(meta *ShareMeta) error {
	data, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return fmt.Errorf("encoding metadata: %w", err)
	}

	if err := os.WriteFile(s.metaPath(meta.ID), data, 0644); err != nil {
		return fmt.Errorf("writing metadata: %w", err)
	}

	return nil
}

// DeleteShare removes a share and its files
func (s *Storage) DeleteShare(id string) error {
	return os.RemoveAll(s.shareDir(id))
}

// CleanupExpired removes all expired shares
func (s *Storage) CleanupExpired() (int, error) {
	sharesDir := filepath.Join(s.dataDir, "shares")
	entries, err := os.ReadDir(sharesDir)
	if err != nil {
		return 0, fmt.Errorf("reading shares directory: %w", err)
	}

	now := time.Now()
	deleted := 0

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		meta, err := s.GetShare(entry.Name())
		if err != nil {
			continue // Skip shares with invalid metadata
		}
		if meta == nil {
			continue
		}

		if meta.ExpiresAt != nil && meta.ExpiresAt.Before(now) {
			if err := s.DeleteShare(meta.ID); err != nil {
				continue // Log but continue with other shares
			}
			deleted++
		}
	}

	return deleted, nil
}
