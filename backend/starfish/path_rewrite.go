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
	"bytes"
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"text/template"
	"time"
)

// PathRewriteRule defines a rule for rewriting object paths
type PathRewriteRule struct {
	Bucket   string `json:"bucket"`   // "*" for all buckets
	Pattern  string `json:"pattern"`  // Regex pattern to match
	Template string `json:"template"` // Go template string
	Priority int    `json:"priority"` // Higher numbers apply first
}

// PathRewriteConfig holds the configuration for path rewriting
type PathRewriteConfig struct {
	Rules []PathRewriteRule `json:"rules"`
}

// TemplateData provides data and functions for template execution
type TemplateData struct {
	Entry StarfishEntry
	// Computed fields for convenience
	ModifyTimeFormatted string
	CreateTimeFormatted string
	AccessTimeFormatted string
	SizeFormatted       string
	// Original key for reference
	OriginalKey string
	// Method wrappers for template access
	GetModifyTimeFormatted func(string) string
	GetCreateTimeFormatted func(string) string
	GetAccessTimeFormatted func(string) string
	GetSizeFormatted       func(string) string
	GetFilenameWithoutExt  func() string
	GetExtension           func() string
	GetParentDir           func() string
	GetVolumeName          func() string
	GetUIDString           func() string
	GetGIDString           func() string
	GetSizeString          func() string
	GetInodeString         func() string
	GetTagsExplicit        func() []string
	GetTagsInherited       func() []string
	GetAllTags             func() []string
}

// applyPathRewrite applies path rewriting rules to an object key
func (b *StarfishBackend) applyPathRewrite(entry StarfishEntry, originalKey string, bucket string) string {
	if b.pathRewriteConfig == nil || len(b.pathRewriteConfig.Rules) == 0 {
		return originalKey
	}

	// Sort rules by priority (highest first)
	sortedRules := make([]PathRewriteRule, len(b.pathRewriteConfig.Rules))
	copy(sortedRules, b.pathRewriteConfig.Rules)
	sort.Slice(sortedRules, func(i, j int) bool {
		return sortedRules[i].Priority > sortedRules[j].Priority
	})

	for _, rule := range sortedRules {
		if rule.Bucket == "*" || rule.Bucket == bucket {
			if matched, newKey := b.applyRule(entry, originalKey, rule); matched {
				return newKey
			}
		}
	}

	return originalKey
}

// applyRule applies a single rewrite rule
func (b *StarfishBackend) applyRule(entry StarfishEntry, originalKey string, rule PathRewriteRule) (bool, string) {
	// Check regex pattern
	re, err := regexp.Compile(rule.Pattern)
	if err != nil {
		return false, originalKey
	}

	if !re.MatchString(originalKey) {
		return false, originalKey
	}

	// Apply template
	newKey, err := b.executeTemplate(entry, rule.Template, originalKey)
	if err != nil {
		return false, originalKey
	}

	return true, newKey
}

// executeTemplate executes a Go template with the entry data
func (b *StarfishBackend) executeTemplate(entry StarfishEntry, templateStr string, originalKey string) (string, error) {
	// Create template data with computed fields
	data := TemplateData{
		Entry:               entry,
		ModifyTimeFormatted: entry.GetModifyTime().Format("2006/01/02"),
		CreateTimeFormatted: entry.GetCreateTime().Format("2006/01/02"),
		AccessTimeFormatted: entry.GetAccessTime().Format("2006/01/02"),
		SizeFormatted:       formatSize(entry.Size),
		OriginalKey:         originalKey,
	}

	// Create template with custom functions
	tmpl, err := template.New("path").Funcs(template.FuncMap{
		// String manipulation
		"join":       strings.Join,
		"split":      strings.Split,
		"lower":      strings.ToLower,
		"upper":      strings.ToUpper,
		"title":      strings.Title,
		"trim":       strings.TrimSpace,
		"trimLeft":   strings.TrimLeft,
		"trimRight":  strings.TrimRight,
		"replace":    strings.Replace,
		"replaceAll": strings.ReplaceAll,
		"hasPrefix":  strings.HasPrefix,
		"hasSuffix":  strings.HasSuffix,
		"contains":   strings.Contains,

		// Path manipulation
		"ext":      filepath.Ext,
		"base":     filepath.Base,
		"dir":      filepath.Dir,
		"clean":    filepath.Clean,
		"joinPath": filepath.Join,

		// Time formatting
		"formatTime": func(t time.Time, layout string) string {
			if t.IsZero() {
				return ""
			}
			return t.Format(layout)
		},
		"formatUnix": func(unixTime int64, layout string) string {
			if unixTime <= 0 {
				return ""
			}
			return time.Unix(unixTime, 0).Format(layout)
		},

		// Size formatting
		"formatSize": func(size int64, unit string) string {
			return formatSizeWithUnit(size, unit)
		},

		// Entry-specific functions
		"getModifyTimeFormatted": func(entry StarfishEntry, layout string) string {
			return entry.GetModifyTimeFormatted(layout)
		},
		"getCreateTimeFormatted": func(entry StarfishEntry, layout string) string {
			return entry.GetCreateTimeFormatted(layout)
		},
		"getAccessTimeFormatted": func(entry StarfishEntry, layout string) string {
			return entry.GetAccessTimeFormatted(layout)
		},
		"getSizeFormatted": func(entry StarfishEntry, unit string) string {
			return entry.GetSizeFormatted(unit)
		},
		"getFilenameWithoutExt": func(entry StarfishEntry) string {
			return entry.GetFilenameWithoutExt()
		},
		"getExtension": func(entry StarfishEntry) string {
			return entry.GetExtension()
		},
		"getParentDir": func(entry StarfishEntry) string {
			return entry.GetParentDir()
		},
		"getVolumeName": func(entry StarfishEntry) string {
			return entry.GetVolumeName()
		},
		"getUIDString": func(entry StarfishEntry) string {
			return entry.GetUIDString()
		},
		"getGIDString": func(entry StarfishEntry) string {
			return entry.GetGIDString()
		},
		"getSizeString": func(entry StarfishEntry) string {
			return entry.GetSizeString()
		},
		"getInodeString": func(entry StarfishEntry) string {
			return entry.GetInodeString()
		},
		"getTagsExplicit": func(entry StarfishEntry) []string {
			return entry.GetTagsExplicit()
		},
		"getTagsInherited": func(entry StarfishEntry) []string {
			return entry.GetTagsInherited()
		},
		"getAllTags": func(entry StarfishEntry) []string {
			return entry.GetAllTags()
		},

		// Array/slice operations
		"first": func(slice []string) string {
			if len(slice) > 0 {
				return slice[0]
			}
			return ""
		},
		"last": func(slice []string) string {
			if len(slice) > 0 {
				return slice[len(slice)-1]
			}
			return ""
		},
		"index": func(slice []string, i int) string {
			if i >= 0 && i < len(slice) {
				return slice[i]
			}
			return ""
		},
		"length": func(slice []string) int {
			return len(slice)
		},

		// Conditional operations
		"if": func(condition bool, trueVal, falseVal string) string {
			if condition {
				return trueVal
			}
			return falseVal
		},
		"default": func(value, defaultValue string) string {
			if value == "" {
				return defaultValue
			}
			return value
		},

		// Mathematical operations
		"add": func(a, b int64) int64 {
			return a + b
		},
		"sub": func(a, b int64) int64 {
			return a - b
		},
		"mul": func(a, b int64) int64 {
			return a * b
		},
		"div": func(a, b int64) int64 {
			if b == 0 {
				return 0
			}
			return a / b
		},

		// Type conversions
		"toString": func(v interface{}) string {
			return fmt.Sprintf("%v", v)
		},
		"toInt": func(v interface{}) int64 {
			switch val := v.(type) {
			case int64:
				return val
			case int:
				return int64(val)
			case string:
				var result int64
				fmt.Sscanf(val, "%d", &result)
				return result
			default:
				return 0
			}
		},
	}).Parse(templateStr)

	if err != nil {
		return "", fmt.Errorf("template parse error: %w", err)
	}

	// Execute template
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("template execution error: %w", err)
	}

	result := buf.String()

	// Clean up the result
	result = strings.TrimSpace(result)
	result = strings.TrimPrefix(result, "/")

	return result, nil
}

// formatSize formats file size in bytes
func formatSize(size int64) string {
	return formatSizeWithUnit(size, "auto")
}

// formatSizeWithUnit formats file size with specified unit
func formatSizeWithUnit(size int64, unit string) string {
	switch unit {
	case "bytes", "b":
		return fmt.Sprintf("%d", size)
	case "kb", "KB":
		return fmt.Sprintf("%d", size/1024)
	case "mb", "MB":
		return fmt.Sprintf("%d", size/(1024*1024))
	case "gb", "GB":
		return fmt.Sprintf("%d", size/(1024*1024*1024))
	case "tb", "TB":
		return fmt.Sprintf("%d", size/(1024*1024*1024*1024))
	case "auto":
		// Auto-format based on size
		switch {
		case size < 1024:
			return fmt.Sprintf("%dB", size)
		case size < 1024*1024:
			return fmt.Sprintf("%dKB", size/1024)
		case size < 1024*1024*1024:
			return fmt.Sprintf("%dMB", size/(1024*1024))
		case size < 1024*1024*1024*1024:
			return fmt.Sprintf("%dGB", size/(1024*1024*1024))
		default:
			return fmt.Sprintf("%dTB", size/(1024*1024*1024*1024))
		}
	default:
		return fmt.Sprintf("%d", size)
	}
}

// Enhanced StarfishEntry methods for template access
func (e *StarfishEntry) GetModifyTimeFormatted(layout string) string {
	if e.ModifyTimeUnix > 0 {
		return time.Unix(e.ModifyTimeUnix, 0).Format(layout)
	}
	return ""
}

func (e *StarfishEntry) GetCreateTimeFormatted(layout string) string {
	if e.CreateTimeUnix > 0 {
		return time.Unix(e.CreateTimeUnix, 0).Format(layout)
	}
	return ""
}

func (e *StarfishEntry) GetAccessTimeFormatted(layout string) string {
	if e.AccessTimeUnix > 0 {
		return time.Unix(e.AccessTimeUnix, 0).Format(layout)
	}
	return ""
}

func (e *StarfishEntry) GetSizeFormatted(unit string) string {
	return formatSizeWithUnit(e.Size, unit)
}

func (e *StarfishEntry) GetFilenameWithoutExt() string {
	return strings.TrimSuffix(e.Filename, filepath.Ext(e.Filename))
}

func (e *StarfishEntry) GetExtension() string {
	return filepath.Ext(e.Filename)
}

func (e *StarfishEntry) GetParentDir() string {
	return filepath.Dir(e.ParentPath)
}

func (e *StarfishEntry) GetVolumeName() string {
	return e.Volume
}

func (e *StarfishEntry) GetUIDString() string {
	return fmt.Sprintf("%d", e.UID)
}

func (e *StarfishEntry) GetGIDString() string {
	return fmt.Sprintf("%d", e.GID)
}

func (e *StarfishEntry) GetSizeString() string {
	return fmt.Sprintf("%d", e.Size)
}

func (e *StarfishEntry) GetInodeString() string {
	return fmt.Sprintf("%d", e.Inode)
}

// GetAllTags returns both explicit and inherited tags
func (e *StarfishEntry) GetAllTags() []string {
	explicit := e.GetTagsExplicit()
	inherited := e.GetTagsInherited()

	// Combine and deduplicate
	tagMap := make(map[string]bool)
	for _, tag := range explicit {
		tagMap[tag] = true
	}
	for _, tag := range inherited {
		tagMap[tag] = true
	}

	result := make([]string, 0, len(tagMap))
	for tag := range tagMap {
		result = append(result, tag)
	}

	return result
}
