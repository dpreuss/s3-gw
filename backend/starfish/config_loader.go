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
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// LoadPathRewriteConfig loads path rewrite configuration from a file
func LoadPathRewriteConfig(configPath string) (*PathRewriteConfig, error) {
	if configPath == "" {
		return nil, nil // No configuration file specified
	}

	// Check if file exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("path rewrite configuration file not found: %s", configPath)
	}

	// Read the configuration file
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read path rewrite configuration file: %w", err)
	}

	// Parse JSON configuration
	var config PathRewriteConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse path rewrite configuration JSON: %w", err)
	}

	// Validate configuration
	if err := validatePathRewriteConfig(&config); err != nil {
		return nil, fmt.Errorf("invalid path rewrite configuration: %w", err)
	}

	return &config, nil
}

// validatePathRewriteConfig validates the path rewrite configuration
func validatePathRewriteConfig(config *PathRewriteConfig) error {
	if config == nil {
		return nil
	}

	for i, rule := range config.Rules {
		// Validate bucket name
		if rule.Bucket == "" {
			return fmt.Errorf("rule %d: bucket name cannot be empty", i)
		}

		// Validate pattern (regex)
		if rule.Pattern == "" {
			return fmt.Errorf("rule %d: pattern cannot be empty", i)
		}

		// Validate template
		if rule.Template == "" {
			return fmt.Errorf("rule %d: template cannot be empty", i)
		}

		// Priority is optional, default to 0 if not specified
		if rule.Priority < 0 {
			return fmt.Errorf("rule %d: priority cannot be negative", i)
		}
	}

	return nil
}

// CreateDefaultPathRewriteConfig creates a default configuration with example rules
func CreateDefaultPathRewriteConfig() *PathRewriteConfig {
	return &PathRewriteConfig{
		Rules: []PathRewriteRule{
			{
				Bucket:   "*",
				Pattern:  "^(.*)$",
				Template: "{{.GetModifyTimeFormatted \"2006/01/02\"}}/{{.Filename}}",
				Priority: 100,
			},
			{
				Bucket:   "Archive",
				Pattern:  "^(.*)$",
				Template: "{{.GetModifyTimeFormatted \"01/02/2006\"}}/{{.Filename}}",
				Priority: 200,
			},
			{
				Bucket:   "Tagged-Data",
				Pattern:  "^(.*)$",
				Template: "{{join .GetTagsExplicit \"/\"}}/{{.Filename}}",
				Priority: 300,
			},
		},
	}
}

// SavePathRewriteConfig saves path rewrite configuration to a file
func SavePathRewriteConfig(config *PathRewriteConfig, configPath string) error {
	if config == nil {
		return fmt.Errorf("configuration cannot be nil")
	}

	// Create directory if it doesn't exist
	dir := filepath.Dir(configPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Marshal to JSON with pretty formatting
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal configuration: %w", err)
	}

	// Write to file
	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write configuration file: %w", err)
	}

	return nil
}
