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
	"path/filepath"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/versity/versitygw/backend"
	"github.com/versity/versitygw/metrics"
	"github.com/versity/versitygw/s3err"
	"github.com/versity/versitygw/s3response"
)

// Ensure StarfishBackend implements backend.Backend interface
var _ backend.Backend = (*StarfishBackend)(nil)

// Starfish-specific error types
type StarfishError struct {
	Code    string
	Message string
	Err     error
}

func (e *StarfishError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("Starfish error [%s]: %s - %v", e.Code, e.Message, e.Err)
	}
	return fmt.Sprintf("Starfish error [%s]: %s", e.Code, e.Message)
}

func (e *StarfishError) Unwrap() error {
	return e.Err
}

// Convert Starfish errors to S3 API errors
func starfishErrToS3Err(err error) error {
	if err == nil {
		return nil
	}

	// Check if it's already an S3 API error
	if _, ok := err.(s3err.APIError); ok {
		return err
	}

	// Check if it's a Starfish error
	var starfishErr *StarfishError
	if fmt.Errorf("", err).Error() == err.Error() {
		// Try to extract Starfish error information
		errStr := err.Error()
		if strings.Contains(errStr, "Starfish error") {
			// Parse Starfish error
			if strings.Contains(errStr, "API_UNAVAILABLE") {
				return s3err.GetAPIError(s3err.ErrInternalError)
			}
			if strings.Contains(errStr, "COLLECTION_NOT_FOUND") {
				return s3err.GetAPIError(s3err.ErrNoSuchBucket)
			}
			if strings.Contains(errStr, "OBJECT_NOT_FOUND") {
				return s3err.GetAPIError(s3err.ErrNoSuchKey)
			}
			if strings.Contains(errStr, "AUTHENTICATION_FAILED") {
				return s3err.GetAPIError(s3err.ErrAccessDenied)
			}
			if strings.Contains(errStr, "RATE_LIMITED") {
				return s3err.GetAPIError(s3err.ErrInternalError)
			}
		}
	}

	// Handle HTTP errors
	if strings.Contains(err.Error(), "connection refused") ||
		strings.Contains(err.Error(), "no such host") {
		return s3err.GetAPIError(s3err.ErrInternalError)
	}

	// Default to internal error
	return s3err.GetAPIError(s3err.ErrInternalError)
}

// Shutdown cleans up resources
func (b *StarfishBackend) Shutdown() {
	if b.cache != nil {
		b.cache.Clear()
	}
}

// String returns a description of the backend
func (b *StarfishBackend) String() string {
	return fmt.Sprintf("StarfishBackend{endpoint: %s}", b.apiEndpoint)
}

// ========== BUCKET OPERATIONS ==========

// ListBuckets returns available buckets based on discovered Collection tags
func (b *StarfishBackend) ListBuckets(_ context.Context,
	_ *s3.ListBucketsInput) (s3response.ListBucketsResult, error) {
	b.collectionsMux.RLock()
	defer b.collectionsMux.RUnlock()

	var buckets []s3response.Bucket
	for bucketName := range b.collections {
		// Create a default creation time for collections
		// In a real implementation, you might want to get this from Starfish metadata
		creationDate := time.Now()
		buckets = append(buckets, s3response.Bucket{
			Name:         &bucketName,
			CreationDate: &creationDate,
		})
	}

	return s3response.ListBucketsResult{
		Buckets: buckets,
	}, nil
}

// HeadBucket checks if a bucket exists
func (b *StarfishBackend) HeadBucket(ctx context.Context, input *s3.HeadBucketInput) (*s3.HeadBucketOutput, error) {
	bucket := *input.Bucket

	b.collectionsMux.RLock()
	defer b.collectionsMux.RUnlock()

	_, exists := b.collections[bucket]
	if !exists {
		return nil, s3err.GetAPIError(s3err.ErrNoSuchBucket)
	}

	return &s3.HeadBucketOutput{}, nil
}

// ========== OBJECT OPERATIONS ==========

// ListObjects implements the S3 ListObjects operation using Starfish query
func (b *StarfishBackend) ListObjects(ctx context.Context, input *s3.ListObjectsInput) (s3response.ListObjectsResult, error) {
	bucket := *input.Bucket
	prefix := ""
	delimiter := ""
	startAfter := ""
	maxKeys := 1000

	if input.Prefix != nil {
		prefix = *input.Prefix
	}
	if input.Delimiter != nil {
		delimiter = *input.Delimiter
	}
	if input.Marker != nil {
		startAfter = *input.Marker
	}
	if input.MaxKeys != nil {
		maxKeys = int(*input.MaxKeys)
	}

	// Debug output
	fmt.Printf("DEBUG: ListObjects called for bucket=%s, prefix=%s, delimiter=%s\n", bucket, prefix, delimiter)

	// Generate cache key
	cacheKey := fmt.Sprintf("v1:%s:%s:%s", bucket, prefix, delimiter)

	// Check cache first
	if cached := b.cache.Get(cacheKey); cached != nil {
		return b.convertToListObjectsResult(cached, bucket, prefix, delimiter, startAfter, maxKeys), nil
	}

	// Build additional query filters
	var additionalFilters []string

	// Only files (f is the type code for regular files)
	additionalFilters = append(additionalFilters, "type=f")

	// Add path/prefix filtering if specified
	if prefix != "" {
		// Add path filtering using parent_path or similar field
		// For now, we'll get all files and filter in post-processing
		// TODO: Optimize with better Starfish query filters for paths
	}

	additionalQuery := strings.Join(additionalFilters, " ")

	// Query Starfish API (volumeAndPath not needed with simplified query endpoint)
	startTime := time.Now()
	result, err := b.QueryStarfish(ctx, bucket, "", additionalQuery)

	// Record metrics
	if b.metricsManager != nil {
		duration := time.Since(startTime).Milliseconds()
		b.metricsManager.Add("starfish_query_duration_ms", duration,
			metrics.Tag{Key: "bucket", Value: bucket},
			metrics.Tag{Key: "operation", Value: "ListObjects"})

		if err != nil {
			b.metricsManager.Add("starfish_query_errors", 1,
				metrics.Tag{Key: "bucket", Value: bucket},
				metrics.Tag{Key: "operation", Value: "ListObjects"})
		} else {
			b.metricsManager.Add("starfish_query_success", 1,
				metrics.Tag{Key: "bucket", Value: bucket},
				metrics.Tag{Key: "operation", Value: "ListObjects"})
			b.metricsManager.Add("starfish_objects_returned", int64(len(result.Entries)),
				metrics.Tag{Key: "bucket", Value: bucket})
		}
	}

	if err != nil {
		return s3response.ListObjectsResult{}, starfishErrToS3Err(err)
	}

	// Cache the result
	b.cache.Set(cacheKey, result, bucket)

	// Convert to S3 ListObjects result
	return b.convertToListObjectsResult(result, bucket, prefix, delimiter, startAfter, maxKeys), nil
}

// ListObjectsV2 implements the S3 ListObjectsV2 operation
func (b *StarfishBackend) ListObjectsV2(ctx context.Context, input *s3.ListObjectsV2Input) (s3response.ListObjectsV2Result, error) {
	bucket := *input.Bucket
	prefix := ""
	delimiter := ""
	startAfter := ""
	maxKeys := 1000

	if input.Prefix != nil {
		prefix = *input.Prefix
	}
	if input.Delimiter != nil {
		delimiter = *input.Delimiter
	}
	if input.StartAfter != nil {
		startAfter = *input.StartAfter
	}
	if input.MaxKeys != nil {
		maxKeys = int(*input.MaxKeys)
	}

	// Debug output
	fmt.Printf("DEBUG: ListObjectsV2 called for bucket=%s, prefix=%s, delimiter=%s\n", bucket, prefix, delimiter)

	// Generate cache key
	cacheKey := fmt.Sprintf("v2:%s:%s:%s", bucket, prefix, delimiter)

	// Check cache first
	if cached := b.cache.Get(cacheKey); cached != nil {
		return b.convertToListObjectsV2Result(cached, bucket, prefix, delimiter, startAfter, maxKeys), nil
	}

	// Build additional query filters
	var additionalFilters []string

	// Only files (f is the type code for regular files)
	additionalFilters = append(additionalFilters, "type=f")

	// Add path/prefix filtering if specified
	if prefix != "" {
		// Add path filtering using parent_path or similar field
		// For now, we'll get all files and filter in post-processing
		// TODO: Optimize with better Starfish query filters for paths
	}

	additionalQuery := strings.Join(additionalFilters, " ")

	// Query Starfish API (volumeAndPath not needed with simplified query endpoint)
	startTime := time.Now()
	result, err := b.QueryStarfish(ctx, bucket, "", additionalQuery)

	// Record metrics
	if b.metricsManager != nil {
		duration := time.Since(startTime).Milliseconds()
		b.metricsManager.Add("starfish_query_duration_ms", duration,
			metrics.Tag{Key: "bucket", Value: bucket},
			metrics.Tag{Key: "operation", Value: "ListObjectsV2"})

		if err != nil {
			b.metricsManager.Add("starfish_query_errors", 1,
				metrics.Tag{Key: "bucket", Value: bucket},
				metrics.Tag{Key: "operation", Value: "ListObjectsV2"})
		} else {
			b.metricsManager.Add("starfish_query_success", 1,
				metrics.Tag{Key: "bucket", Value: bucket},
				metrics.Tag{Key: "operation", Value: "ListObjectsV2"})
			b.metricsManager.Add("starfish_objects_returned", int64(len(result.Entries)),
				metrics.Tag{Key: "bucket", Value: bucket})
		}
	}

	if err != nil {
		return s3response.ListObjectsV2Result{}, starfishErrToS3Err(err)
	}

	// Cache the result
	b.cache.Set(cacheKey, result, bucket)

	// Convert to S3 ListObjectsV2 result
	return b.convertToListObjectsV2Result(result, bucket, prefix, delimiter, startAfter, maxKeys), nil
}

// convertToListObjectsResult converts Starfish entries to S3 ListObjects format
func (b *StarfishBackend) convertToListObjectsResult(starfishResult *StarfishQueryResponse, bucket, prefix, delimiter, startAfter string, maxKeys int) s3response.ListObjectsResult {
	var contents []s3response.Object
	var commonPrefixes []types.CommonPrefix
	isTruncated := false

	count := 0
	for _, entry := range starfishResult.Entries {
		if maxKeys > 0 && count >= maxKeys {
			isTruncated = true
			break
		}

		// Build S3 object key from Starfish entry with bucket context
		objectKey := b.buildObjectKeyFromEntryWithBucket(entry, bucket)

		// Skip if before startAfter
		if startAfter != "" && objectKey <= startAfter {
			continue
		}

		// Handle delimiter logic for common prefixes
		if delimiter != "" && b.shouldBeCommonPrefix(objectKey, prefix, delimiter) {
			commonPrefix := b.getCommonPrefix(objectKey, prefix, delimiter)
			if !b.containsCommonPrefix(commonPrefixes, commonPrefix) {
				commonPrefixes = append(commonPrefixes, types.CommonPrefix{
					Prefix: &commonPrefix,
				})
			}
			continue
		}

		// Add as regular object
		eTag := b.generateETag(entry)
		modifyTime := entry.GetModifyTime()
		contents = append(contents, s3response.Object{
			Key:          &objectKey,
			Size:         &entry.Size,
			LastModified: &modifyTime,
			ETag:         &eTag,
		})
		count++
	}

	bucketName := bucket

	maxKeysPtr := int32(maxKeys)
	return s3response.ListObjectsResult{
		Contents:       contents,
		CommonPrefixes: commonPrefixes,
		IsTruncated:    &isTruncated,
		MaxKeys:        &maxKeysPtr,
		Name:           &bucketName,
		Prefix:         &prefix,
		Delimiter:      &delimiter,
	}
}

// convertToListObjectsV2Result converts Starfish entries to S3 ListObjectsV2 format
func (b *StarfishBackend) convertToListObjectsV2Result(starfishResult *StarfishQueryResponse, bucket, prefix, delimiter, startAfter string, maxKeys int) s3response.ListObjectsV2Result {
	var contents []s3response.Object
	var commonPrefixes []types.CommonPrefix
	isTruncated := false

	count := 0
	for _, entry := range starfishResult.Entries {
		if maxKeys > 0 && count >= maxKeys {
			isTruncated = true
			break
		}

		// Build S3 object key from Starfish entry with bucket context
		objectKey := b.buildObjectKeyFromEntryWithBucket(entry, bucket)

		// Skip if before startAfter
		if startAfter != "" && objectKey <= startAfter {
			continue
		}

		// Handle delimiter logic for common prefixes
		if delimiter != "" && b.shouldBeCommonPrefix(objectKey, prefix, delimiter) {
			commonPrefix := b.getCommonPrefix(objectKey, prefix, delimiter)
			if !b.containsCommonPrefix(commonPrefixes, commonPrefix) {
				commonPrefixes = append(commonPrefixes, types.CommonPrefix{
					Prefix: &commonPrefix,
				})
			}
			continue
		}

		// Add as regular object
		eTag := b.generateETag(entry)
		modifyTime := entry.GetModifyTime()
		contents = append(contents, s3response.Object{
			Key:          &objectKey,
			Size:         &entry.Size,
			LastModified: &modifyTime,
			ETag:         &eTag,
		})
		count++
	}

	bucketName := bucket
	maxKeysPtr := int32(maxKeys)
	keyCount := int32(len(contents))

	return s3response.ListObjectsV2Result{
		Contents:       contents,
		CommonPrefixes: commonPrefixes,
		IsTruncated:    &isTruncated,
		MaxKeys:        &maxKeysPtr,
		Name:           &bucketName,
		Prefix:         &prefix,
		Delimiter:      &delimiter,
		KeyCount:       &keyCount,
		StartAfter:     &startAfter,
	}
}

// HeadObject retrieves metadata for a single object
func (b *StarfishBackend) HeadObject(ctx context.Context, input *s3.HeadObjectInput) (*s3.HeadObjectOutput, error) {
	bucket := *input.Bucket
	object := *input.Key

	// Query Starfish to get file metadata
	additionalQuery := "type=f"

	result, err := b.QueryStarfish(ctx, bucket, "", additionalQuery)
	if err != nil {
		return nil, starfishErrToS3Err(err)
	}

	// Find the matching file by object key in the results
	var foundEntry *StarfishEntry
	for _, entry := range result.Entries {
		entryKey := b.buildObjectKeyFromEntryWithBucket(entry, bucket)
		if entryKey == object {
			foundEntry = &entry
			break
		}
	}

	if foundEntry == nil {
		return nil, s3err.GetAPIError(s3err.ErrNoSuchKey)
	}

	entry := *foundEntry

	// Build response
	eTag := b.generateETag(entry)
	modifyTime := entry.GetModifyTime()
	contentLength := entry.Size

	return &s3.HeadObjectOutput{
		ETag:          &eTag,
		LastModified:  &modifyTime,
		ContentLength: &contentLength,
	}, nil
}

// GetObject retrieves object content via the starfish file server
func (b *StarfishBackend) GetObject(ctx context.Context, input *s3.GetObjectInput) (*s3.GetObjectOutput, error) {
	bucket := *input.Bucket
	object := *input.Key

	// Check if file server is configured
	if b.fileServerURL == "" {
		return nil, s3err.GetAPIError(s3err.ErrNotImplemented)
	}

	// Get file metadata first using HeadObject
	headInput := &s3.HeadObjectInput{
		Bucket: input.Bucket,
		Key:    input.Key,
	}

	headResult, err := b.HeadObject(ctx, headInput)
	if err != nil {
		return nil, err
	}

	// Query Starfish to get volume and path information for the file
	additionalQuery := "type=f"

	result, err := b.QueryStarfish(ctx, bucket, "", additionalQuery)
	if err != nil {
		return nil, starfishErrToS3Err(err)
	}

	// Find the matching file by object key in the results
	var foundEntry *StarfishEntry
	for _, entry := range result.Entries {
		entryKey := b.buildObjectKeyFromEntryWithBucket(entry, bucket)
		if entryKey == object {
			foundEntry = &entry
			break
		}
	}

	if foundEntry == nil {
		return nil, s3err.GetAPIError(s3err.ErrNoSuchKey)
	}

	entry := *foundEntry

	// Get the file content from the file server
	// URL format: {fileServerURL}/{volume}/{path}
	filePath := object
	if entry.FullPath != "" {
		filePath = strings.TrimPrefix(entry.FullPath, "/")
	}

	fileURL := fmt.Sprintf("%s/%s/%s", b.fileServerURL, entry.Volume, filePath)

	// Make HTTP request to file server
	req, err := http.NewRequestWithContext(ctx, "GET", fileURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create file server request: %w", err)
	}

	// Add internal security header for file server authentication
	req.Header.Set("X-Internal-Token", b.bearerToken)

	resp, err := b.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch file from file server: %w", err)
	}

	if resp.StatusCode == http.StatusNotFound {
		return nil, s3err.GetAPIError(s3err.ErrNoSuchKey)
	}

	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("file server returned status %d", resp.StatusCode)
	}

	// Return the response body as the object content
	return &s3.GetObjectOutput{
		Body:          resp.Body,
		ETag:          headResult.ETag,
		LastModified:  headResult.LastModified,
		ContentLength: headResult.ContentLength,
	}, nil
}

// ========== ACCESS CONTROL INTEGRATION ==========

// GetBucketAcl retrieves the ACL for a bucket (collection)
func (b *StarfishBackend) GetBucketAcl(ctx context.Context, input *s3.GetBucketAclInput) ([]byte, error) {
	bucket := *input.Bucket

	// Check if bucket exists
	b.collectionsMux.RLock()
	_, exists := b.collections[bucket]
	b.collectionsMux.RUnlock()

	if !exists {
		return nil, s3err.GetAPIError(s3err.ErrNoSuchBucket)
	}

	// For now, return a default ACL that grants full control to the bucket owner
	// In a real implementation, you might want to store ACLs in Starfish metadata
	defaultACL := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<AccessControlPolicy xmlns="http://s3.amazonaws.com/doc/2006-03-01/">
  <Owner>
    <ID>starfish-collection-owner</ID>
    <DisplayName>Starfish Collection Owner</DisplayName>
  </Owner>
  <AccessControlList>
    <Grant>
      <Grantee xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xsi:type="CanonicalUser">
        <ID>starfish-collection-owner</ID>
        <DisplayName>Starfish Collection Owner</DisplayName>
      </Grantee>
      <Permission>FULL_CONTROL</Permission>
    </Grant>
  </AccessControlList>
</AccessControlPolicy>`)

	return []byte(defaultACL), nil
}

// PutBucketAcl sets the ACL for a bucket (collection)
func (b *StarfishBackend) PutBucketAcl(ctx context.Context, bucket string, data []byte) error {
	// Check if bucket exists
	b.collectionsMux.RLock()
	_, exists := b.collections[bucket]
	b.collectionsMux.RUnlock()

	if !exists {
		return s3err.GetAPIError(s3err.ErrNoSuchBucket)
	}

	// For now, we'll just validate that the ACL is well-formed XML
	// In a real implementation, you might want to store ACLs in Starfish metadata
	if len(data) == 0 {
		return fmt.Errorf("empty ACL data")
	}

	// Basic XML validation - check if it contains required elements
	aclStr := string(data)
	if !strings.Contains(aclStr, "<AccessControlPolicy>") {
		return fmt.Errorf("invalid ACL format: missing AccessControlPolicy")
	}

	// TODO: Store ACL in Starfish metadata or external storage
	// For now, we'll just accept the ACL without storing it
	fmt.Printf("DEBUG: ACL for bucket %s would be stored: %s\n", bucket, string(data))

	return nil
}

// GetBucketPolicy retrieves the bucket policy for a collection
func (b *StarfishBackend) GetBucketPolicy(ctx context.Context, input *s3.GetBucketPolicyInput) ([]byte, error) {
	bucket := *input.Bucket

	// Check if bucket exists
	b.collectionsMux.RLock()
	_, exists := b.collections[bucket]
	b.collectionsMux.RUnlock()

	if !exists {
		return nil, s3err.GetAPIError(s3err.ErrNoSuchBucket)
	}

	// For now, return no policy (which means no bucket policy exists)
	// In a real implementation, you might want to store policies in Starfish metadata
	return nil, s3err.GetAPIError(s3err.ErrNoSuchBucketPolicy)
}

// PutBucketPolicy sets the bucket policy for a collection
func (b *StarfishBackend) PutBucketPolicy(ctx context.Context, bucket string, data []byte) error {
	// Check if bucket exists
	b.collectionsMux.RLock()
	_, exists := b.collections[bucket]
	b.collectionsMux.RUnlock()

	if !exists {
		return s3err.GetAPIError(s3err.ErrNoSuchBucket)
	}

	// Validate JSON policy
	var policy map[string]interface{}
	if err := json.Unmarshal(data, &policy); err != nil {
		return fmt.Errorf("invalid bucket policy JSON: %w", err)
	}

	// TODO: Store policy in Starfish metadata or external storage
	// For now, we'll just accept the policy without storing it
	fmt.Printf("DEBUG: Policy for bucket %s would be stored: %s\n", bucket, string(data))

	return nil
}

// DeleteBucketPolicy deletes the bucket policy for a collection
func (b *StarfishBackend) DeleteBucketPolicy(ctx context.Context, input *s3.DeleteBucketPolicyInput) error {
	bucket := *input.Bucket

	// Check if bucket exists
	b.collectionsMux.RLock()
	_, exists := b.collections[bucket]
	b.collectionsMux.RUnlock()

	if !exists {
		return s3err.GetAPIError(s3err.ErrNoSuchBucket)
	}

	// TODO: Remove policy from Starfish metadata or external storage
	fmt.Printf("DEBUG: Policy for bucket %s would be deleted\n", bucket)

	return nil
}

// ========== HELPER METHODS ==========

// buildObjectKeyFromEntryWithBucket builds an S3 object key from a Starfish entry
func (b *StarfishBackend) buildObjectKeyFromEntryWithBucket(entry StarfishEntry, bucket string) string {
	// If we have a full path, use it
	if entry.FullPath != "" {
		return strings.TrimPrefix(entry.FullPath, "/")
	}

	// Otherwise, build from parent path and filename
	if entry.ParentPath != "" {
		return filepath.Join(entry.ParentPath, entry.Filename)
	}

	return entry.Filename
}

// shouldBeCommonPrefix determines if an object should be treated as a common prefix
func (b *StarfishBackend) shouldBeCommonPrefix(objectKey, prefix, delimiter string) bool {
	if !strings.HasPrefix(objectKey, prefix) {
		return false
	}

	// Find the delimiter after the prefix
	afterPrefix := objectKey[len(prefix):]
	delimiterIndex := strings.Index(afterPrefix, delimiter)
	if delimiterIndex == -1 {
		return false
	}

	// Check if there's more content after the delimiter
	return delimiterIndex+len(delimiter) < len(afterPrefix)
}

// getCommonPrefix extracts the common prefix from an object key
func (b *StarfishBackend) getCommonPrefix(objectKey, prefix, delimiter string) string {
	afterPrefix := objectKey[len(prefix):]
	delimiterIndex := strings.Index(afterPrefix, delimiter)
	if delimiterIndex == -1 {
		return prefix
	}

	return prefix + afterPrefix[:delimiterIndex+len(delimiter)]
}

// containsCommonPrefix checks if a common prefix already exists in the list
func (b *StarfishBackend) containsCommonPrefix(commonPrefixes []types.CommonPrefix, newPrefix string) bool {
	for _, cp := range commonPrefixes {
		if *cp.Prefix == newPrefix {
			return true
		}
	}
	return false
}

// generateETag generates an ETag for a Starfish entry
func (b *StarfishBackend) generateETag(entry StarfishEntry) string {
	// For now, use a simple ETag based on file size and modification time
	// In a real implementation, you might want to calculate an MD5 hash
	return fmt.Sprintf("\"%d-%d\"", entry.Size, entry.ModifyTimeUnix)
}

// InitializeCollections discovers Collections: tagset tags from Starfish
func (b *StarfishBackend) InitializeCollections(ctx context.Context) error {
	// Query Starfish for all Collections: tags
	queryURL := fmt.Sprintf("%s/tagsets/Collections:/tags", b.apiEndpoint)

	req, err := http.NewRequestWithContext(ctx, "GET", queryURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create collections request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+b.bearerToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := b.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to fetch collections: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("collections API returned status %d: %s", resp.StatusCode, string(body))
	}

	// Parse the response - expect an array of tag names
	var tagNames []string
	if err := json.NewDecoder(resp.Body).Decode(&tagNames); err != nil {
		return fmt.Errorf("failed to decode collections response: %w", err)
	}

	// Update the collections map
	b.collectionsMux.Lock()
	defer b.collectionsMux.Unlock()

	// Clear existing collections and add new ones
	b.collections = make(map[string]string)
	for _, tagName := range tagNames {
		// The tag name becomes the bucket name
		b.collections[tagName] = fmt.Sprintf("Collections:%s", tagName)
	}

	fmt.Printf("DEBUG: Discovered %d collections: %v\n", len(b.collections), tagNames)
	return nil
}

// GetCollectionTag returns the Collections: tagset tag for a given bucket name
func (b *StarfishBackend) GetCollectionTag(bucketName string) (string, bool) {
	b.collectionsMux.RLock()
	defer b.collectionsMux.RUnlock()

	tag, exists := b.collections[bucketName]
	return tag, exists
}

// GetAllCollections returns all discovered collections
func (b *StarfishBackend) GetAllCollections() map[string]string {
	b.collectionsMux.RLock()
	defer b.collectionsMux.RUnlock()

	// Return a copy to avoid race conditions
	result := make(map[string]string)
	for k, v := range b.collections {
		result[k] = v
	}
	return result
}
