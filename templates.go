package main

import (
	"embed"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"strings"
)

//go:embed templates/*.html
var templateFS embed.FS

//go:embed static/*
var staticFS embed.FS

// Templates holds parsed templates
type Templates struct {
	upload   *template.Template
	download *template.Template
}

// LoadTemplates parses all templates
func LoadTemplates() (*Templates, error) {
	upload, err := template.ParseFS(templateFS, "templates/upload.html")
	if err != nil {
		return nil, fmt.Errorf("parsing upload template: %w", err)
	}

	download, err := template.ParseFS(templateFS, "templates/download.html")
	if err != nil {
		return nil, fmt.Errorf("parsing download template: %w", err)
	}

	return &Templates{
		upload:   upload,
		download: download,
	}, nil
}

// DownloadPageData is the data passed to the download template
type DownloadPageData struct {
	ID                string
	FileName          string
	FileSize          int64
	FileSizeFormatted string
	ExpiresAt         string
	PasswordRequired  bool
}

func formatFileSize(bytes int64) string {
	if bytes < 1024 {
		return fmt.Sprintf("%d B", bytes)
	}
	if bytes < 1024*1024 {
		return fmt.Sprintf("%.1f KB", float64(bytes)/1024)
	}
	if bytes < 1024*1024*1024 {
		return fmt.Sprintf("%.1f MB", float64(bytes)/(1024*1024))
	}
	return fmt.Sprintf("%.2f GB", float64(bytes)/(1024*1024*1024))
}

// HandleUploadPage serves the upload page
func (h *Handlers) HandleUploadPage(w http.ResponseWriter, r *http.Request, tmpl *Templates) {
	if err := tmpl.upload.Execute(w, nil); err != nil {
		log.Printf("Error rendering upload page: %v", err)
		http.Error(w, "Internal error", http.StatusInternalServerError)
	}
}

// HandleDownloadPage serves the download page
func (h *Handlers) HandleDownloadPage(w http.ResponseWriter, r *http.Request, tmpl *Templates) {
	// Extract ID from path like /s/abc123
	id := strings.TrimPrefix(r.URL.Path, "/s/")
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

	data := DownloadPageData{
		ID:                meta.ID,
		FileName:          meta.FileName,
		FileSize:          meta.FileSize,
		FileSizeFormatted: formatFileSize(meta.FileSize),
		PasswordRequired:  meta.PasswordHash != "",
	}

	if meta.ExpiresAt != nil {
		data.ExpiresAt = meta.ExpiresAt.Format("Jan 2, 2006")
	}

	if err := tmpl.download.Execute(w, data); err != nil {
		log.Printf("Error rendering download page: %v", err)
		http.Error(w, "Internal error", http.StatusInternalServerError)
	}
}
