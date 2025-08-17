// Copyright (c) 2025 Starfish Storage, Inc.
//
// This file is part of the VersityGW project developed by Starfish Storage, Inc.
// This file was assisted by Gemini AI.
//
// The VersityGW project is licensed under the Apache License, version 2.0
// (the "License"); you may not use this file except in compliance with the
// License. You may obtain a copy of the License at:
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package starfish

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"time"
)

// NewStarfishBackend creates a new Starfish backend instance
func NewStarfishBackend(config *StarfishConfig) (*StarfishBackend, error) {
	if config.APIEndpoint == "" {
		return nil, fmt.Errorf("API endpoint is required")
	}
	if config.BearerToken == "" {
		return nil, fmt.Errorf("bearer token is required")
	}

	if config.CacheTTL == 0 {
		config.CacheTTL = time.Hour // Default 1 hour cache
	}
	if config.CollectionsRefreshInterval == 0 {
		config.CollectionsRefreshInterval = 10 * time.Minute
	}

	// Configure TLS
	tlsConfig := &tls.Config{
		InsecureSkipVerify: config.TLSInsecureSkipVerify,
	}

	// Set minimum TLS version
	if config.TLSMinVersion != "" {
		switch config.TLSMinVersion {
		case "1.2":
			tlsConfig.MinVersion = tls.VersionTLS12
		case "1.3":
			tlsConfig.MinVersion = tls.VersionTLS13
		default:
			return nil, fmt.Errorf("unsupported TLS version: %s (supported: 1.2, 1.3)", config.TLSMinVersion)
		}
	} else {
		// Default to TLS 1.2
		tlsConfig.MinVersion = tls.VersionTLS12
	}

	// Load client certificates if provided
	if config.TLSCertFile != "" && config.TLSKeyFile != "" {
		cert, err := tls.LoadX509KeyPair(config.TLSCertFile, config.TLSKeyFile)
		if err != nil {
			return nil, fmt.Errorf("failed to load TLS certificate: %w", err)
		}
		tlsConfig.Certificates = []tls.Certificate{cert}
	}

	// Create HTTP client with TLS configuration and connection pooling
	httpClient := &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig:     tlsConfig,
			MaxIdleConns:        100,              // Maximum idle connections
			MaxIdleConnsPerHost: 10,               // Maximum idle connections per host
			IdleConnTimeout:     90 * time.Second, // How long to keep idle connections
			TLSHandshakeTimeout: 10 * time.Second, // TLS handshake timeout
			DisableCompression:  false,            // Enable compression
			ForceAttemptHTTP2:   true,             // Enable HTTP/2
		},
	}

	backend := &StarfishBackend{
		apiEndpoint:                config.APIEndpoint,
		bearerToken:                config.BearerToken,
		fileServerURL:              config.FileServerURL,
		cache:                      NewQueryCache(config.CacheTTL, config.MetricsManager),
		httpClient:                 httpClient,
		collections:                make(map[string]string),
		CollectionsRefreshInterval: config.CollectionsRefreshInterval,
		pathRewriteConfig:          config.PathRewriteConfig,
		metricsManager:             config.MetricsManager,
	}

	return backend, nil
}

// Note: InitializeCollections is implemented in starfish.go - discovers Collections: tagset

// GetCollectionTag returns the Collections: tagset tag for a given bucket name
func (b *StarfishBackend) GetCollectionTag(bucketName string) (string, bool) {
	b.collectionsMux.RLock()
	defer b.collectionsMux.RUnlock()

	tag, exists := b.collections[bucketName]
	return tag, exists
}

// AddCollection adds a new collection mapping
func (b *StarfishBackend) AddCollection(bucketName, collectionTag string) {
	b.collectionsMux.Lock()
	defer b.collectionsMux.Unlock()

	b.collections[bucketName] = collectionTag
}

// GetAllCollections returns all discovered collections
func (b *StarfishBackend) GetAllCollections() map[string]string {
	b.collectionsMux.RLock()
	defer b.collectionsMux.RUnlock()

	// Return a copy to avoid race conditions
	result := make(map[string]string)
	for bucket, tag := range b.collections {
		result[bucket] = tag
	}
	return result
}
