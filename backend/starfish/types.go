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
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/versity/versitygw/metrics"
)

// StarfishBackend implements the backend.Backend interface for Starfish API
type StarfishBackend struct {
	apiEndpoint                string
	bearerToken                string
	fileServerURL              string // URL to the starfish file server for GetObject operations
	cache                      *QueryCache
	httpClient                 *http.Client
	collections                map[string]string  // maps bucket name -> Collection:* tag
	collectionsMux             sync.RWMutex       // protects collections map
	CollectionsRefreshInterval time.Duration      // interval for refreshing collections
	pathRewriteConfig          *PathRewriteConfig // path rewriting configuration
	metricsManager             *metrics.Manager   // Metrics manager for monitoring
}

// StarfishConfig holds configuration for the backend
type StarfishConfig struct {
	APIEndpoint                string
	BearerToken                string
	FileServerURL              string // URL to the starfish file server for GetObject operations
	CacheTTL                   time.Duration
	CollectionsRefreshInterval time.Duration      // interval for refreshing collections
	PathRewriteConfig          *PathRewriteConfig // path rewriting configuration

	// TLS Configuration
	TLSCertFile           string // Path to TLS certificate file
	TLSKeyFile            string // Path to TLS private key file
	TLSInsecureSkipVerify bool   // Skip TLS certificate verification (for testing)
	TLSMinVersion         string // Minimum TLS version (e.g., "1.2", "1.3")

	// Performance & Monitoring
	MetricsManager      *metrics.Manager // Metrics manager for monitoring
	ConnectionPoolSize  int              // Number of connections in pool (default: 100)
	MaxIdleConnsPerHost int              // Max idle connections per host (default: 10)
	IdleConnTimeout     time.Duration    // Idle connection timeout (default: 90s)
}

// QueryCache manages cached query results
type QueryCache struct {
	data       map[string]*CachedResult
	mutex      sync.RWMutex
	defaultTTL time.Duration
}

// CachedResult stores a Starfish query result with metadata
type CachedResult struct {
	Data          *StarfishQueryResponse
	CachedAt      time.Time
	ExpiresAt     time.Time
	VolumeAndPath string
}

// StarfishQueryResponse represents the response from Starfish query API
type StarfishQueryResponse struct {
	Entries []StarfishEntry `json:"entries"`
	Total   int             `json:"total"`
}

// StarfishEntry represents a single file/directory entry from Starfish API response
type StarfishEntry struct {
	ID               int            `json:"_id"`
	Filename         string         `json:"fn"`
	ParentPath       string         `json:"parent_path,omitempty"`
	FullPath         string         `json:"full_path,omitempty"`
	Type             int            `json:"type"` // 32768 for file, directory type differs
	Size             int64          `json:"size"`
	Mode             string         `json:"mode,omitempty"`
	UID              int            `json:"uid,omitempty"`
	GID              int            `json:"gid,omitempty"`
	CreateTimeUnix   int64          `json:"ct,omitempty"`
	ModifyTimeUnix   int64          `json:"mt,omitempty"`
	AccessTimeUnix   int64          `json:"at,omitempty"`
	Volume           string         `json:"volume"`
	Inode            int64          `json:"ino,omitempty"`
	SizeUnit         string         `json:"size_unit,omitempty"`
	TagsExplicitStr  string         `json:"tags_explicit,omitempty"`
	TagsInheritedStr string         `json:"tags_inherited,omitempty"`
	Zones            []StarfishZone `json:"zones,omitempty"`
}

// GetCreateTime converts Unix timestamp to time.Time
func (e *StarfishEntry) GetCreateTime() time.Time {
	if e.CreateTimeUnix > 0 {
		return time.Unix(e.CreateTimeUnix, 0)
	}
	return time.Time{}
}

// GetModifyTime converts Unix timestamp to time.Time
func (e *StarfishEntry) GetModifyTime() time.Time {
	if e.ModifyTimeUnix > 0 {
		return time.Unix(e.ModifyTimeUnix, 0)
	}
	return time.Time{}
}

// GetAccessTime converts Unix timestamp to time.Time
func (e *StarfishEntry) GetAccessTime() time.Time {
	if e.AccessTimeUnix > 0 {
		return time.Unix(e.AccessTimeUnix, 0)
	}
	return time.Time{}
}

// IsFile checks if the entry is a file (type 32768)
func (e *StarfishEntry) IsFile() bool {
	return e.Type == 32768
}

// GetTagsExplicit parses the tags_explicit string into a slice
func (e *StarfishEntry) GetTagsExplicit() []string {
	if e.TagsExplicitStr == "" {
		return []string{}
	}
	return strings.Split(e.TagsExplicitStr, ",")
}

// GetTagsInherited parses the tags_inherited string into a slice
func (e *StarfishEntry) GetTagsInherited() []string {
	if e.TagsInheritedStr == "" {
		return []string{}
	}
	return strings.Split(e.TagsInheritedStr, ",")
}

// StarfishZone represents zone information from Starfish
type StarfishZone struct {
	ID           int    `json:"id"`
	Name         string `json:"name"`
	RelativePath string `json:"relative_path"`
}

// StarfishTagsResponse represents the response from the tags API (legacy)
type StarfishTagsResponse struct {
	Tags []string `json:"tags"`
}

// StarfishTagsetResponse represents the response from the /tagset/{tagset_name}/ API
type StarfishTagsetResponse struct {
	TagNames []StarfishTagName `json:"tag_names"`
}

// StarfishTagName represents a tag name in the tagset response
type StarfishTagName struct {
	Name string `json:"name"`
}

// StarfishTag represents a single tag (kept for backward compatibility)
type StarfishTag struct {
	Name string `json:"name"`
}
