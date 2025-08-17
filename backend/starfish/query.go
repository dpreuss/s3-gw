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
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// QueryStarfish executes a query against the Starfish API using Collections: tagset tags
func (b *StarfishBackend) QueryStarfish(ctx context.Context, bucket, volumeAndPath, additionalQuery string) (*StarfishQueryResponse, error) {
	// Get the Collections: tag for this bucket
	collectionTag, exists := b.GetCollectionTag(bucket)
	if !exists {
		return nil, fmt.Errorf("no Collections: tag found for bucket: %s", bucket)
	}

	// Build the query URL using the Collections: tag and volume path
	queryURL, err := b.buildQueryURL(collectionTag, volumeAndPath, additionalQuery)
	if err != nil {
		return nil, fmt.Errorf("failed to build query URL: %w", err)
	}

	// Debug output
	fmt.Printf("DEBUG: QueryStarfish URL: %s\n", queryURL)

	// Create HTTP request
	req, err := http.NewRequestWithContext(ctx, "GET", queryURL, nil)
	if err != nil {
		return nil, &StarfishError{
			Code:    "REQUEST_CREATION_FAILED",
			Message: "Failed to create HTTP request",
			Err:     err,
		}
	}

	// Add authentication header
	req.Header.Set("Authorization", "Bearer "+b.bearerToken)
	req.Header.Set("Content-Type", "application/json")

	// Execute request
	fmt.Printf("DEBUG: Making HTTP request to Starfish API...\n")
	resp, err := b.httpClient.Do(req)
	if err != nil {
		fmt.Printf("DEBUG: HTTP request failed: %v\n", err)
		return nil, &StarfishError{
			Code:    "API_UNAVAILABLE",
			Message: "Starfish API is unavailable",
			Err:     err,
		}
	}
	defer resp.Body.Close()

	fmt.Printf("DEBUG: HTTP response status: %d\n", resp.StatusCode)

	// Check response status
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		fmt.Printf("DEBUG: API error response: %s\n", string(body))
		var errorCode string
		switch resp.StatusCode {
		case http.StatusUnauthorized:
			errorCode = "AUTHENTICATION_FAILED"
		case http.StatusNotFound:
			errorCode = "COLLECTION_NOT_FOUND"
		case http.StatusTooManyRequests:
			errorCode = "RATE_LIMITED"
		default:
			errorCode = "API_ERROR"
		}
		return nil, &StarfishError{
			Code:    errorCode,
			Message: fmt.Sprintf("API request failed with status %d: %s", resp.StatusCode, string(body)),
		}
	}

	// Parse response - Starfish returns an array of entries directly
	fmt.Printf("DEBUG: Parsing JSON response...\n")
	var entries []StarfishEntry
	if err := json.NewDecoder(resp.Body).Decode(&entries); err != nil {
		fmt.Printf("DEBUG: JSON decode failed: %v\n", err)
		return nil, &StarfishError{
			Code:    "RESPONSE_DECODE_FAILED",
			Message: "Failed to decode API response",
			Err:     err,
		}
	}

	// Convert to our response format
	result := &StarfishQueryResponse{
		Entries: entries,
		Total:   len(entries),
	}

	// Debug output
	fmt.Printf("DEBUG: QueryStarfish returned %d entries\n", len(entries))

	return result, nil
}

// buildQueryURL constructs the Starfish query URL using the simple /query/ endpoint
func (b *StarfishBackend) buildQueryURL(collectionTag, volumeAndPath, additionalQuery string) (string, error) {
	// Build base URL: /query/
	baseURL := fmt.Sprintf("%s/query/",
		strings.TrimSuffix(b.apiEndpoint, "/"))

	// Build query parameters
	params := url.Values{}

	// Build query filter using Collections: tagset tag and additional filters
	var queryFilters []string

	// Add Collections: tagset tag filter - use the full "Collections:TagName" format
	queryFilters = append(queryFilters, fmt.Sprintf("tag=%s", collectionTag))

	// Add additional query filters if provided
	if additionalQuery != "" {
		queryFilters = append(queryFilters, additionalQuery)
	}

	// Combine filters with space separation (as shown in API docs)
	if len(queryFilters) > 0 {
		params.Set("query", strings.Join(queryFilters, " "))
	}

	// Set format to include necessary fields for S3 compatibility
	params.Set("format", "parent_path fn type size ct mt at uid gid mode volume tags_explicit tags_inherited")

	// Set reasonable limit (can be made configurable later)
	params.Set("limit", "1000")

	// Set sort order for consistent results
	params.Set("sort_by", "parent_path,fn")

	// Combine URL and parameters
	if len(params) > 0 {
		baseURL += "?" + params.Encode()
	}

	return baseURL, nil
}

// QueryStarfishTags discovers all tags in the specified tagset using /tagset/{tagset_name}/ endpoint
func (b *StarfishBackend) QueryStarfishTags(ctx context.Context, tagset string) (*StarfishTagsResponse, error) {
	// Build the tagset query URL
	queryURL, err := b.buildTagsQueryURL(tagset)
	if err != nil {
		return nil, fmt.Errorf("failed to build tagset query URL: %w", err)
	}

	// Create HTTP request
	req, err := http.NewRequestWithContext(ctx, "GET", queryURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create tagset request: %w", err)
	}

	// Add authentication header
	req.Header.Set("Authorization", "Bearer "+b.bearerToken)
	req.Header.Set("Content-Type", "application/json")

	// Execute request
	resp, err := b.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute tagset request: %w", err)
	}
	defer resp.Body.Close()

	// Check response status
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, &ErrStarfishAPIAccess{
			StatusCode: resp.StatusCode,
			Msg:        string(body),
		}
	}

	// Parse response using tagset format
	var tagsetResult StarfishTagsetResponse
	if err := json.NewDecoder(resp.Body).Decode(&tagsetResult); err != nil {
		return nil, fmt.Errorf("failed to decode tagset response: %w", err)
	}

	// Convert to legacy format for compatibility
	var tags []string
	for _, tagName := range tagsetResult.TagNames {
		tags = append(tags, tagName.Name)
	}

	result := &StarfishTagsResponse{
		Tags: tags,
	}

	return result, nil
}

// buildTagsQueryURL constructs the Starfish tagset query URL using /tagset/{tagset_name}/ endpoint
func (b *StarfishBackend) buildTagsQueryURL(tagset string) (string, error) {
	// Build base URL for tagset endpoint: /tagset/{tagset_name}/
	baseURL := fmt.Sprintf("%s/tagset/%s/",
		strings.TrimSuffix(b.apiEndpoint, "/"),
		tagset)

	// Build query parameters (if needed)
	params := url.Values{}

	// Set a reasonable limit (can be made configurable later)
	params.Set("limit", "1000")

	// Include private tags
	params.Set("with_private", "true")

	// Combine URL and parameters
	if len(params) > 0 {
		baseURL += "?" + params.Encode()
	}

	return baseURL, nil
}

// s3PathToStarfishPath converts S3-style paths to Starfish volume:path format
func (b *StarfishBackend) s3PathToStarfishPath(bucket, prefix string) string {
	// TODO: This needs to be updated based on how Collection tags map to actual volumes
	// For now, using bucket as volume name, but this will likely need to change
	// based on the Collection -> volume mapping

	if prefix == "" {
		return fmt.Sprintf("%s:", bucket)
	}

	// Replace / with %2F for Starfish API
	starfishPath := strings.ReplaceAll(prefix, "/", "%2F")
	return fmt.Sprintf("%s:%s", bucket, starfishPath)
}
