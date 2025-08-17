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
	"sync"
	"time"

	"github.com/versity/versitygw/metrics"
)

// QueryCache provides caching for Starfish query results
type QueryCache struct {
	data       map[string]*CachedResult
	mutex      sync.RWMutex
	defaultTTL time.Duration

	// Metrics integration
	metricsManager *metrics.Manager
}

// CachedResult represents a cached query result
type CachedResult struct {
	Data          *StarfishQueryResponse
	CachedAt      time.Time
	ExpiresAt     time.Time
	VolumeAndPath string
	HitCount      int64 // Track cache hit count for metrics
}

// NewQueryCache creates a new query cache with optional metrics integration
func NewQueryCache(defaultTTL time.Duration, metricsManager *metrics.Manager) *QueryCache {
	return &QueryCache{
		data:           make(map[string]*CachedResult),
		defaultTTL:     defaultTTL,
		metricsManager: metricsManager,
	}
}

// Get retrieves a cached result if it exists and hasn't expired
func (c *QueryCache) Get(key string) *StarfishQueryResponse {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	cached, exists := c.data[key]
	if !exists {
		// Cache miss - record metric
		if c.metricsManager != nil {
			c.metricsManager.Add("starfish_cache_miss", 1,
				metrics.Tag{Key: "cache_key", Value: key})
		}
		return nil
	}

	if time.Now().After(cached.ExpiresAt) {
		// Expired, remove from cache
		delete(c.data, key)
		// Cache miss due to expiration - record metric
		if c.metricsManager != nil {
			c.metricsManager.Add("starfish_cache_miss", 1,
				metrics.Tag{Key: "cache_key", Value: key},
				metrics.Tag{Key: "reason", Value: "expired"})
		}
		return nil
	}

	// Cache hit - increment hit count and record metric
	cached.HitCount++
	if c.metricsManager != nil {
		c.metricsManager.Add("starfish_cache_hit", 1,
			metrics.Tag{Key: "cache_key", Value: key})
		c.metricsManager.Add("starfish_cache_hit_count", cached.HitCount,
			metrics.Tag{Key: "cache_key", Value: key})
	}

	return cached.Data
}

// Set stores a result in the cache
func (c *QueryCache) Set(key string, data *StarfishQueryResponse, volumeAndPath string) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	now := time.Now()
	c.data[key] = &CachedResult{
		Data:          data,
		CachedAt:      now,
		ExpiresAt:     now.Add(c.defaultTTL),
		VolumeAndPath: volumeAndPath,
		HitCount:      0,
	}

	// Record cache set metric
	if c.metricsManager != nil {
		c.metricsManager.Add("starfish_cache_set", 1,
			metrics.Tag{Key: "cache_key", Value: key})
		c.metricsManager.Add("starfish_cache_entries", int64(len(c.data)))
	}
}

// Invalidate removes a specific cache entry
func (c *QueryCache) Invalidate(key string) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if _, exists := c.data[key]; exists {
		delete(c.data, key)
		// Record cache invalidation metric
		if c.metricsManager != nil {
			c.metricsManager.Add("starfish_cache_invalidate", 1,
				metrics.Tag{Key: "cache_key", Value: key})
			c.metricsManager.Add("starfish_cache_entries", int64(len(c.data)))
		}
	}
}

// Clear removes all cache entries
func (c *QueryCache) Clear() {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	clearedCount := len(c.data)
	c.data = make(map[string]*CachedResult)

	// Record cache clear metric
	if c.metricsManager != nil {
		c.metricsManager.Add("starfish_cache_clear", int64(clearedCount))
		c.metricsManager.Add("starfish_cache_entries", 0)
	}
}

// Stats returns cache statistics
func (c *QueryCache) Stats() map[string]interface{} {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	valid := 0
	expired := 0
	totalHits := int64(0)
	now := time.Now()

	for _, cached := range c.data {
		if now.After(cached.ExpiresAt) {
			expired++
		} else {
			valid++
			totalHits += cached.HitCount
		}
	}

	stats := map[string]interface{}{
		"total_entries":   len(c.data),
		"valid_entries":   valid,
		"expired_entries": expired,
		"total_hits":      totalHits,
		"cache_hit_ratio": 0.0,
	}

	// Calculate hit ratio if we have data
	if len(c.data) > 0 {
		stats["cache_hit_ratio"] = float64(totalHits) / float64(len(c.data))
	}

	return stats
}

// GetCacheMetrics returns metrics for monitoring systems
func (c *QueryCache) GetCacheMetrics() map[string]int64 {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	valid := 0
	expired := 0
	totalHits := int64(0)
	now := time.Now()

	for _, cached := range c.data {
		if now.After(cached.ExpiresAt) {
			expired++
		} else {
			valid++
			totalHits += cached.HitCount
		}
	}

	return map[string]int64{
		"starfish_cache_total_entries":   int64(len(c.data)),
		"starfish_cache_valid_entries":   int64(valid),
		"starfish_cache_expired_entries": int64(expired),
		"starfish_cache_total_hits":      totalHits,
	}
}
