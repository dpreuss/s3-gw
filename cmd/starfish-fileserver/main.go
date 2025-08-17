// Copyright (c) 2025 Starfish Storage, Inc.
// SPDX-License-Identifier: BUSL-1.1

package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// VolumeInfo represents volume information from Starfish API
type VolumeInfo struct {
	ID          int               `json:"id"`
	Vol         string            `json:"vol"`
	DisplayName string            `json:"display_name"`
	Mounts      map[string]string `json:"mounts"`
	MountOpts   map[string]string `json:"mount_opts"`
	Type        string            `json:"type"`
}

// FileServer provides HTTP access to files via Starfish volume mappings
type FileServer struct {
	starfishEndpoint string
	starfishToken    string
	httpClient       *http.Client
	volumes          map[string]*VolumeInfo // vol name -> VolumeInfo
	volumesMux       sync.RWMutex
	port             int
}

// NewFileServer creates a new file server instance
func NewFileServer(starfishEndpoint, starfishToken string, port int) *FileServer {
	return &FileServer{
		starfishEndpoint: starfishEndpoint,
		starfishToken:    starfishToken,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		volumes: make(map[string]*VolumeInfo),
		port:    port,
	}
}

// LoadVolumes fetches volume information from Starfish API
func (fs *FileServer) LoadVolumes() error {
	url := fmt.Sprintf("%s/volume/?sort_by=display_name", fs.starfishEndpoint)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+fs.starfishToken)
	req.Header.Set("Accept", "application/json")

	resp, err := fs.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API error: status %d, body: %s", resp.StatusCode, string(body))
	}

	var volumes []VolumeInfo
	if err := json.NewDecoder(resp.Body).Decode(&volumes); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	fs.volumesMux.Lock()
	defer fs.volumesMux.Unlock()

	// Clear existing volumes
	fs.volumes = make(map[string]*VolumeInfo)

	// Add new volumes
	for i := range volumes {
		vol := &volumes[i]
		fs.volumes[vol.Vol] = vol
		log.Printf("Loaded volume: %s -> %v", vol.Vol, vol.Mounts)
	}

	log.Printf("Loaded %d volumes from Starfish API", len(volumes))
	return nil
}

// ResolvePath converts a Starfish volume:path to a local filesystem path
func (fs *FileServer) ResolvePath(volumeName, filePath string) (string, error) {
	fs.volumesMux.RLock()
	defer fs.volumesMux.RUnlock()

	volume, exists := fs.volumes[volumeName]
	if !exists {
		return "", fmt.Errorf("volume not found: %s", volumeName)
	}

	// For now, use the first available mount point
	// In a production system, you might want to choose based on availability or other criteria
	for agentAddr, mountPath := range volume.Mounts {
		log.Printf("Using mount: %s -> %s for volume %s", agentAddr, mountPath, volumeName)

		// Clean the file path and join with mount path
		cleanPath := filepath.Clean(filePath)
		if strings.HasPrefix(cleanPath, "/") {
			cleanPath = cleanPath[1:] // Remove leading slash
		}

		fullPath := filepath.Join(mountPath, cleanPath)
		return fullPath, nil
	}

	return "", fmt.Errorf("no mounts available for volume: %s", volumeName)
}

// ServeFile handles file serving requests
func (fs *FileServer) ServeFile(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse path: /volume/path/to/file
	pathParts := strings.SplitN(strings.TrimPrefix(r.URL.Path, "/"), "/", 2)
	if len(pathParts) < 2 {
		http.Error(w, "Invalid path format. Expected: /volume/path/to/file", http.StatusBadRequest)
		return
	}

	volumeName := pathParts[0]
	filePath := "/" + pathParts[1]

	log.Printf("Serving file: volume=%s, path=%s", volumeName, filePath)

	// Resolve to local filesystem path
	localPath, err := fs.ResolvePath(volumeName, filePath)
	if err != nil {
		log.Printf("Path resolution failed: %v", err)
		http.Error(w, fmt.Sprintf("Path resolution failed: %v", err), http.StatusNotFound)
		return
	}

	log.Printf("Resolved to local path: %s", localPath)

	// Check if file exists and is readable
	fileInfo, err := os.Stat(localPath)
	if err != nil {
		if os.IsNotExist(err) {
			log.Printf("File not found: %s", localPath)
			http.Error(w, "File not found", http.StatusNotFound)
		} else {
			log.Printf("File stat error: %v", err)
			http.Error(w, "File access error", http.StatusInternalServerError)
		}
		return
	}

	// Ensure it's a regular file
	if !fileInfo.Mode().IsRegular() {
		log.Printf("Not a regular file: %s", localPath)
		http.Error(w, "Not a regular file", http.StatusBadRequest)
		return
	}

	// Open and serve the file
	file, err := os.Open(localPath)
	if err != nil {
		log.Printf("Failed to open file: %v", err)
		http.Error(w, "Failed to open file", http.StatusInternalServerError)
		return
	}
	defer file.Close()

	// Set appropriate headers
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Length", fmt.Sprintf("%d", fileInfo.Size()))
	w.Header().Set("Last-Modified", fileInfo.ModTime().UTC().Format(http.TimeFormat))

	// Copy file content to response
	_, err = io.Copy(w, file)
	if err != nil {
		log.Printf("Failed to send file content: %v", err)
		return
	}

	log.Printf("Successfully served file: %s (%d bytes)", localPath, fileInfo.Size())
}

// HealthCheck handles health check requests
func (fs *FileServer) HealthCheck(w http.ResponseWriter, r *http.Request) {
	response := map[string]interface{}{
		"status":         "healthy",
		"volumes_loaded": len(fs.volumes),
		"timestamp":      time.Now().UTC().Format(time.RFC3339),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// Start starts the HTTP server
func (fs *FileServer) Start() error {
	mux := http.NewServeMux()

	// File serving endpoint
	mux.HandleFunc("/", fs.ServeFile)

	// Health check endpoint
	mux.HandleFunc("/health", fs.HealthCheck)

	addr := fmt.Sprintf(":%d", fs.port)
	log.Printf("Starting Starfish File Server on %s", addr)
	log.Printf("File serving endpoint: http://localhost%s/{volume}/{path/to/file}", addr)
	log.Printf("Health check endpoint: http://localhost%s/health", addr)

	return http.ListenAndServe(addr, mux)
}

func main() {
	var (
		endpoint = flag.String("endpoint", "", "Starfish API endpoint (required)")
		token    = flag.String("token", "", "Starfish API token (required)")
		port     = flag.Int("port", 8080, "Port to listen on")
	)
	flag.Parse()

	if *endpoint == "" || *token == "" {
		fmt.Fprintf(os.Stderr, "Usage: %s -endpoint <starfish-api-endpoint> -token <api-token> [-port <port>]\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "\nExample:\n")
		fmt.Fprintf(os.Stderr, "  %s -endpoint https://sf-redashdev.sfish.dev/api -token \"sf-api-v1:...\" -port 8080\n", os.Args[0])
		os.Exit(1)
	}

	// Create file server
	fs := NewFileServer(*endpoint, *token, *port)

	// Load volume information
	log.Printf("Loading volume information from Starfish API...")
	if err := fs.LoadVolumes(); err != nil {
		log.Fatalf("Failed to load volumes: %v", err)
	}

	// Start HTTP server
	log.Fatal(fs.Start())
}
