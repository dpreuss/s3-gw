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
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/versity/versitygw/s3response"
)

// Test file for starfish backend

// Mock Starfish API server for testing
func newTestServer(handler http.HandlerFunc) *httptest.Server {
	return httptest.NewServer(handler)
}

// Helper to create a new backend for testing
func newTestBackend(serverURL string) (*StarfishBackend, error) {
	backend, err := NewStarfishBackend(&StarfishConfig{
		APIEndpoint: serverURL,
		BearerToken: "test-token",
		CacheTTL:    1 * time.Minute,
	})
	if err != nil {
		return nil, err
	}

	// Add mock collections for testing
	backend.AddCollection("test-bucket", "Collections:TestCollection")
	backend.AddCollection("another-bucket", "Collections:AnotherCollection")

	return backend, nil
}

func TestListObjects(t *testing.T) {
	server := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		// Mock response for ListObjects - return array directly as per API docs
		entries := []StarfishEntry{
			{Filename: "file1.txt", ParentPath: "/", Size: 1024, ModifyTimeUnix: time.Now().Unix(), Volume: "test-volume"},
			{Filename: "file2.txt", ParentPath: "/", Size: 2048, ModifyTimeUnix: time.Now().Unix(), Volume: "test-volume"},
		}
		json.NewEncoder(w).Encode(entries)
	})
	defer server.Close()

	backend, err := newTestBackend(server.URL)
	if err != nil {
		t.Fatalf("failed to create test backend: %v", err)
	}

	bucket := "test-bucket"
	input := &s3.ListObjectsInput{
		Bucket: &bucket,
	}

	result, err := backend.ListObjects(context.Background(), input)
	if err != nil {
		t.Fatalf("ListObjects failed: %v", err)
	}

	if len(result.Contents) != 2 {
		t.Errorf("expected 2 objects, got %d", len(result.Contents))
	}
	if result.Contents[0].Key != nil && *result.Contents[0].Key != "file1.txt" {
		t.Errorf("expected object key 'file1.txt', got '%s'", *result.Contents[0].Key)
	}
}

func TestListObjectsV2(t *testing.T) {
	server := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		// Mock response for ListObjectsV2 - return array directly as per API docs
		entries := []StarfishEntry{
			{Filename: "file1.txt", ParentPath: "/", Size: 1024, ModifyTimeUnix: time.Now().Unix(), Volume: "test-volume"},
			{Filename: "file2.txt", ParentPath: "/", Size: 2048, ModifyTimeUnix: time.Now().Unix(), Volume: "test-volume"},
		}
		json.NewEncoder(w).Encode(entries)
	})
	defer server.Close()

	backend, err := newTestBackend(server.URL)
	if err != nil {
		t.Fatalf("failed to create test backend: %v", err)
	}

	bucket := "test-bucket"
	input := &s3.ListObjectsV2Input{
		Bucket: &bucket,
	}

	result, err := backend.ListObjectsV2(context.Background(), input)
	if err != nil {
		t.Fatalf("ListObjectsV2 failed: %v", err)
	}

	if len(result.Contents) != 2 {
		t.Errorf("expected 2 objects, got %d", len(result.Contents))
	}
	if result.Contents[0].Key != nil && *result.Contents[0].Key != "file1.txt" {
		t.Errorf("expected object key 'file1.txt', got '%s'", *result.Contents[0].Key)
	}
}

func TestHeadObject(t *testing.T) {
	server := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		// Mock response for HeadObject - return array directly as per API docs
		entries := []StarfishEntry{
			{Filename: "file1.txt", ParentPath: "/", Size: 1024, ModifyTimeUnix: time.Now().Unix(), Volume: "test-volume"},
		}
		json.NewEncoder(w).Encode(entries)
	})
	defer server.Close()

	backend, err := newTestBackend(server.URL)
	if err != nil {
		t.Fatalf("failed to create test backend: %v", err)
	}

	bucket := "test-bucket"
	key := "file1.txt"
	input := &s3.HeadObjectInput{
		Bucket: &bucket,
		Key:    &key,
	}

	result, err := backend.HeadObject(context.Background(), input)
	if err != nil {
		t.Fatalf("HeadObject failed: %v", err)
	}

	if result.ContentLength == nil || *result.ContentLength != 1024 {
		t.Errorf("expected size 1024, got %d", *result.ContentLength)
	}
}

func TestHeadObjectNotFound(t *testing.T) {
	server := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		// Mock response for object not found - return empty array
		entries := []StarfishEntry{}
		json.NewEncoder(w).Encode(entries)
	})
	defer server.Close()

	backend, err := newTestBackend(server.URL)
	if err != nil {
		t.Fatalf("failed to create test backend: %v", err)
	}

	bucket := "test-bucket"
	key := "non-existent-file.txt"
	input := &s3.HeadObjectInput{
		Bucket: &bucket,
		Key:    &key,
	}

	_, err = backend.HeadObject(context.Background(), input)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	// Check if it's the right type of error - just check that we got an error
	// In practice, this would be a specific NoSuchKey error
	if err == nil {
		t.Error("expected error but got nil")
	}
}

func TestGetObjectAttributes(t *testing.T) {
	server := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		// Mock response for GetObjectAttributes - return array directly as per API docs
		entries := []StarfishEntry{
			{Filename: "file1.txt", ParentPath: "/", Size: 1024, ModifyTimeUnix: time.Now().Unix(), Volume: "test-volume"},
		}
		json.NewEncoder(w).Encode(entries)
	})
	defer server.Close()

	backend, err := newTestBackend(server.URL)
	if err != nil {
		t.Fatalf("failed to create test backend: %v", err)
	}

	bucket := "test-bucket"
	key := "file1.txt"
	input := &s3.GetObjectAttributesInput{
		Bucket: &bucket,
		Key:    &key,
	}

	result, err := backend.GetObjectAttributes(context.Background(), input)
	if err != nil {
		t.Fatalf("GetObjectAttributes failed: %v", err)
	}

	if result.ObjectSize == nil || *result.ObjectSize != 1024 {
		t.Errorf("expected size 1024, got %d", *result.ObjectSize)
	}
}

func TestUnsupportedOperations(t *testing.T) {
	backend, err := newTestBackend("http://test.example.com")
	if err != nil {
		t.Fatalf("failed to create test backend: %v", err)
	}

	// Test PutObject
	putInput := s3response.PutObjectInput{}
	_, err = backend.PutObject(context.Background(), putInput)
	if err == nil {
		t.Error("expected error for PutObject, got nil")
	}

	// Test GetObject
	bucket := "test-bucket"
	key := "test-key"
	getInput := &s3.GetObjectInput{
		Bucket: &bucket,
		Key:    &key,
	}
	_, err = backend.GetObject(context.Background(), getInput)
	if err == nil {
		t.Error("expected error for GetObject, got nil")
	}

	// Test DeleteObject
	deleteInput := &s3.DeleteObjectInput{
		Bucket: &bucket,
		Key:    &key,
	}
	_, err = backend.DeleteObject(context.Background(), deleteInput)
	if err == nil {
		t.Error("expected error for DeleteObject, got nil")
	}
}

func TestCacheOperations(t *testing.T) {
	backend, err := newTestBackend("http://test.example.com")
	if err != nil {
		t.Fatalf("failed to create test backend: %v", err)
	}

	// Test cache operations
	cache := backend.cache

	// Test empty cache
	result := cache.Get("test-key")
	if result != nil {
		t.Error("expected nil from empty cache")
	}

	// Test cache set and get
	testData := &StarfishQueryResponse{
		Entries: []StarfishEntry{
			{Filename: "test.txt", Size: 100},
		},
	}
	cache.Set("test-key", testData, "test-volume:/test/path")

	result = cache.Get("test-key")
	if result == nil {
		t.Error("expected cached result, got nil")
	}

	if len(result.Entries) != 1 || result.Entries[0].Filename != "test.txt" {
		t.Error("cached result doesn't match expected data")
	}

	// Test cache stats
	stats := cache.Stats()
	if stats["total_entries"].(int) != 1 {
		t.Errorf("expected 1 total entry, got %d", stats["total_entries"].(int))
	}
}

func TestPathConversion(t *testing.T) {
	backend, err := newTestBackend("http://test.example.com")
	if err != nil {
		t.Fatalf("failed to create test backend: %v", err)
	}

	// Test s3PathToStarfishPath
	tests := []struct {
		bucket   string
		prefix   string
		expected string
	}{
		{"mybucket", "", "mybucket:"},
		{"mybucket", "folder", "mybucket:folder"},
		{"mybucket", "folder/subfolder", "mybucket:folder%2Fsubfolder"},
		{"mybucket", "a/b/c", "mybucket:a%2Fb%2Fc"},
	}

	for _, tt := range tests {
		result := backend.s3PathToStarfishPath(tt.bucket, tt.prefix)
		if result != tt.expected {
			t.Errorf("s3PathToStarfishPath(%s, %s) = %s, expected %s",
				tt.bucket, tt.prefix, result, tt.expected)
		}
	}
}

func TestConfigValidation(t *testing.T) {
	// Test missing endpoint
	_, err := NewStarfishBackend(&StarfishConfig{
		BearerToken: "token",
	})
	if err == nil {
		t.Error("expected error for missing endpoint")
	}

	// Test missing token
	_, err = NewStarfishBackend(&StarfishConfig{
		APIEndpoint: "http://test.example.com",
	})
	if err == nil {
		t.Error("expected error for missing token")
	}

	// Test valid config
	_, err = NewStarfishBackend(&StarfishConfig{
		APIEndpoint: "http://test.example.com",
		BearerToken: "token",
		CacheTTL:    time.Hour,
	})
	if err != nil {
		t.Errorf("expected no error for valid config, got %v", err)
	}
}

func TestInitializeCollections(t *testing.T) {
	server := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		// Mock response for tags endpoint - return tags from Collections: tagset
		response := StarfishTagsResponse{
			Tags: []string{
				"ProjectA",
				"ProjectB",
				"DataArchive",
				"TestCollection",
			},
		}
		json.NewEncoder(w).Encode(response)
	})
	defer server.Close()

	backend, err := newTestBackend(server.URL)
	if err != nil {
		t.Fatalf("Failed to create backend: %v", err)
	}

	// Clear any existing collections to test fresh initialization
	backend.collections = make(map[string]string)

	// Initialize collections
	ctx := context.Background()
	err = backend.InitializeCollections(ctx)
	if err != nil {
		t.Fatalf("InitializeCollections failed: %v", err)
	}

	// Verify the collections were properly discovered and mapped
	collections := backend.GetAllCollections()
	expectedCollections := map[string]string{
		"projecta":       "Collections:ProjectA",
		"projectb":       "Collections:ProjectB",
		"dataarchive":    "Collections:DataArchive",
		"testcollection": "Collections:TestCollection",
	}

	if len(collections) != len(expectedCollections) {
		t.Fatalf("Expected %d collections, got %d", len(expectedCollections), len(collections))
	}

	for bucketName, expectedTag := range expectedCollections {
		actualTag, exists := collections[bucketName]
		if !exists {
			t.Errorf("Expected bucket %s not found in collections", bucketName)
		}
		if actualTag != expectedTag {
			t.Errorf("Expected bucket %s to map to %s, got %s", bucketName, expectedTag, actualTag)
		}
	}

	// Test that all tags have the Collections: prefix
	for bucketName := range collections {
		tag := collections[bucketName]
		if !strings.HasPrefix(tag, "Collections:") {
			t.Errorf("Tag without Collections: prefix found: %s -> %s", bucketName, tag)
		}
	}
}
