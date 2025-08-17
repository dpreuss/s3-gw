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
	"testing"
	"time"
)

func TestPathRewrite(t *testing.T) {
	// Create a test backend
	backend := &StarfishBackend{
		pathRewriteConfig: &PathRewriteConfig{
			Rules: []PathRewriteRule{
				{
					Bucket:   "*",
					Pattern:  "^(.*)$",
					Template: "{{getModifyTimeFormatted .Entry \"2006/01/02\"}}/{{.Entry.Filename}}",
					Priority: 100,
				},
				{
					Bucket:   "Archive",
					Pattern:  "^(.*)$",
					Template: "{{getModifyTimeFormatted .Entry \"01/02/2006\"}}/{{.Entry.Filename}}",
					Priority: 200,
				},
				{
					Bucket:   "Tagged-Data",
					Pattern:  "^(.*)$",
					Template: "{{join (getTagsExplicit .Entry) \"/\"}}/{{.Entry.Filename}}",
					Priority: 300,
				},
			},
		},
	}

	// Test entry with modification time
	testTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)
	entry := StarfishEntry{
		Filename:        "document.pdf",
		ModifyTimeUnix:  testTime.Unix(),
		Size:            1048576,
		UID:             1000,
		GID:             1000,
		Volume:          "storage1",
		TagsExplicitStr: "project-a,important",
	}

	// Test default bucket (should use first rule)
	result := backend.applyPathRewrite(entry, "original/path/document.pdf", "default")
	expected := "2024/01/15/document.pdf"
	if result != expected {
		t.Errorf("Expected %s, got %s", expected, result)
	}

	// Test Archive bucket (should use second rule)
	result = backend.applyPathRewrite(entry, "original/path/document.pdf", "Archive")
	expected = "01/15/2024/document.pdf"
	if result != expected {
		t.Errorf("Expected %s, got %s", expected, result)
	}

	// Test Tagged-Data bucket (should use third rule)
	result = backend.applyPathRewrite(entry, "original/path/document.pdf", "Tagged-Data")
	expected = "project-a/important/document.pdf"
	if result != expected {
		t.Errorf("Expected %s, got %s", expected, result)
	}
}

func TestTemplateFunctions(t *testing.T) {
	backend := &StarfishBackend{}

	testTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)
	entry := StarfishEntry{
		Filename:        "document.pdf",
		ModifyTimeUnix:  testTime.Unix(),
		Size:            1048576,
		UID:             1000,
		GID:             1000,
		Volume:          "storage1",
		TagsExplicitStr: "project-a,important",
	}

	// Test various template functions
	tests := []struct {
		template string
		expected string
	}{
		{
			template: "{{getModifyTimeFormatted .Entry \"2006/01/02\"}}",
			expected: "2024/01/15",
		},
		{
			template: "{{getSizeFormatted .Entry \"mb\"}}",
			expected: "1",
		},
		{
			template: "{{getFilenameWithoutExt .Entry}}",
			expected: "document",
		},
		{
			template: "{{getExtension .Entry}}",
			expected: ".pdf",
		},
		{
			template: "{{join (getTagsExplicit .Entry) \"/\"}}",
			expected: "project-a/important",
		},
		{
			template: "{{lower .Entry.Filename}}",
			expected: "document.pdf",
		},
		{
			template: "{{upper .Entry.Filename}}",
			expected: "DOCUMENT.PDF",
		},
	}

	for i, test := range tests {
		result, err := backend.executeTemplate(entry, test.template, "original/path")
		if err != nil {
			t.Errorf("Test %d: Template execution failed: %v", i, err)
			continue
		}
		if result != test.expected {
			t.Errorf("Test %d: Expected %s, got %s", i, test.expected, result)
		}
	}
}

func TestPathRewriteConfigValidation(t *testing.T) {
	// Test valid configuration
	validConfig := &PathRewriteConfig{
		Rules: []PathRewriteRule{
			{
				Bucket:   "*",
				Pattern:  "^(.*)$",
				Template: "{{.Filename}}",
				Priority: 100,
			},
		},
	}

	if err := validatePathRewriteConfig(validConfig); err != nil {
		t.Errorf("Valid config should not error: %v", err)
	}

	// Test invalid configuration (empty bucket)
	invalidConfig := &PathRewriteConfig{
		Rules: []PathRewriteRule{
			{
				Bucket:   "",
				Pattern:  "^(.*)$",
				Template: "{{.Filename}}",
				Priority: 100,
			},
		},
	}

	if err := validatePathRewriteConfig(invalidConfig); err == nil {
		t.Error("Invalid config should error")
	}
}

func TestFormatSize(t *testing.T) {
	tests := []struct {
		size     int64
		unit     string
		expected string
	}{
		{1024, "kb", "1"},
		{1048576, "mb", "1"},
		{1073741824, "gb", "1"},
		{1099511627776, "tb", "1"},
		{1024, "auto", "1KB"},
		{1048576, "auto", "1MB"},
		{1073741824, "auto", "1GB"},
		{1099511627776, "auto", "1TB"},
	}

	for i, test := range tests {
		result := formatSizeWithUnit(test.size, test.unit)
		if result != test.expected {
			t.Errorf("Test %d: Expected %s, got %s", i, test.expected, result)
		}
	}
}
