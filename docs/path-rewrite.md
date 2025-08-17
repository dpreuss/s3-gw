# Starfish S3 Gateway Path Rewrite Feature

## Overview

The Starfish S3 Gateway includes a powerful path rewriting feature that allows you to transform object keys (paths) based on file metadata using Go's built-in template engine. This feature enables dynamic organization of your S3 objects without moving or copying the underlying files.

## Features

- **Flexible Template System**: Uses Go's `text/template` engine with custom functions
- **Metadata-Driven**: Access to all Starfish file metadata (dates, sizes, tags, etc.)
- **Multiple Date Formats**: Support for various date formats (YYYY/MM/DD, MM/DD/YYYY, DD/MM/YYYY, etc.)
- **Size Formatting**: Automatic size formatting (bytes, KB, MB, GB, TB)
- **Tag Support**: Handle multi-valued tags with join, first, last operations
- **Conditional Logic**: Support for if/else conditions in templates
- **Priority-Based Rules**: Multiple rules with priority ordering
- **Per-Bucket Configuration**: Different rules for different buckets

## Configuration

### Command Line Usage

```bash
versitygw starfish \
  --endpoint http://starfish-api:8080 \
  --token your-bearer-token \
  --path-rewrite-config /path/to/rewrite-rules.json
```

### Environment Variable

```bash
export VGW_STARFISH_PATH_REWRITE_CONFIG=/path/to/rewrite-rules.json
versitygw starfish --endpoint http://starfish-api:8080 --token your-bearer-token
```

### Configuration File Format

The configuration file is in JSON format:

```json
{
  "rules": [
    {
      "bucket": "*",
      "pattern": "^(.*)$",
      "template": "{{.GetModifyTimeFormatted \"2006/01/02\"}}/{{.Filename}}",
      "priority": 100
    }
  ]
}
```

## Template Variables

### Basic File Information

| Variable | Description | Example |
|----------|-------------|---------|
| `{{.Filename}}` | Original filename | `document.pdf` |
| `{{.ParentPath}}` | Parent directory path | `/data/projects` |
| `{{.FullPath}}` | Full file path | `/data/projects/document.pdf` |
| `{{.Volume}}` | Starfish volume name | `storage1` |
| `{{.Size}}` | File size in bytes | `1048576` |
| `{{.UID}}` | User ID | `1000` |
| `{{.GID}}` | Group ID | `1000` |

### Time Formatting

| Variable | Description | Example |
|----------|-------------|---------|
| `{{getModifyTimeFormatted .Entry "2006/01/02"}}` | Modification time (YYYY/MM/DD) | `2024/01/15` |
| `{{getModifyTimeFormatted .Entry "01/02/2006"}}` | Modification time (MM/DD/YYYY) | `01/15/2024` |
| `{{getModifyTimeFormatted .Entry "02/01/2006"}}` | Modification time (DD/MM/YYYY) | `15/01/2024` |
| `{{getCreateTimeFormatted .Entry "2006/01/02"}}` | Creation time (YYYY/MM/DD) | `2024/01/10` |
| `{{getAccessTimeFormatted .Entry "2006/01/02"}}` | Access time (YYYY/MM/DD) | `2024/01/20` |

### Size Formatting

| Variable | Description | Example |
|----------|-------------|---------|
| `{{getSizeFormatted .Entry "bytes"}}` | Size in bytes | `1048576` |
| `{{getSizeFormatted .Entry "kb"}}` | Size in KB | `1024` |
| `{{getSizeFormatted .Entry "mb"}}` | Size in MB | `1` |
| `{{getSizeFormatted .Entry "gb"}}` | Size in GB | `1` |
| `{{getSizeFormatted .Entry "auto"}}` | Auto-formatted size | `1MB` |

### Path Manipulation

| Variable | Description | Example |
|----------|-------------|---------|
| `{{getFilenameWithoutExt .Entry}}` | Filename without extension | `document` |
| `{{getExtension .Entry}}` | File extension | `.pdf` |
| `{{getParentDir .Entry}}` | Parent directory | `/data/projects` |

### Tag Operations

| Variable | Description | Example |
|----------|-------------|---------|
| `{{getTagsExplicit .Entry}}` | Explicit tags as slice | `["project-a", "important"]` |
| `{{getTagsInherited .Entry}}` | Inherited tags as slice | `["department-eng"]` |
| `{{getAllTags .Entry}}` | All tags (explicit + inherited) | `["project-a", "important", "department-eng"]` |

## Template Functions

### String Functions

| Function | Description | Example |
|----------|-------------|---------|
| `{{join (getTagsExplicit .Entry) "/"}}` | Join array with separator | `project-a/important` |
| `{{split "a/b/c" "/"}}` | Split string into array | `["a", "b", "c"]` |
| `{{lower .Entry.Filename}}` | Convert to lowercase | `document.pdf` |
| `{{upper .Entry.Filename}}` | Convert to uppercase | `DOCUMENT.PDF` |
| `{{title .Entry.Filename}}` | Title case | `Document.Pdf` |
| `{{trim .Entry.Filename}}` | Trim whitespace | `document.pdf` |
| `{{replace .Entry.Filename "old" "new"}}` | Replace substring | `newdocument.pdf` |

### Path Functions

| Function | Description | Example |
|----------|-------------|---------|
| `{{ext .Entry.Filename}}` | Get file extension | `.pdf` |
| `{{base .Entry.FullPath}}` | Get base filename | `document.pdf` |
| `{{dir .Entry.FullPath}}` | Get directory path | `/data/projects` |
| `{{clean .Entry.FullPath}}` | Clean path | `/data/projects/document.pdf` |
| `{{joinPath "a" "b" "c"}}` | Join path components | `a/b/c` |

### Array Functions

| Function | Description | Example |
|----------|-------------|---------|
| `{{first (getTagsExplicit .Entry)}}` | First element | `project-a` |
| `{{last (getTagsExplicit .Entry)}}` | Last element | `important` |
| `{{index (getTagsExplicit .Entry) 1}}` | Element at index | `important` |
| `{{length (getTagsExplicit .Entry)}}` | Array length | `2` |

### Conditional Functions

| Function | Description | Example |
|----------|-------------|---------|
| `{{if (gt .Entry.Size 1073741824)}}large{{else}}small{{end}}` | Conditional logic | `large` |
| `{{default (first (getTagsExplicit .Entry)) "untagged"}}` | Default value | `untagged` |

### Mathematical Functions

| Function | Description | Example |
|----------|-------------|---------|
| `{{add .Entry.Size 1024}}` | Addition | `1049600` |
| `{{sub .Entry.Size 1024}}` | Subtraction | `1047552` |
| `{{mul .Entry.Size 2}}` | Multiplication | `2097152` |
| `{{div .Entry.Size 1024}}` | Division | `1024` |

## Configuration Examples

### Time-Based Organization

```json
{
  "rules": [
    {
      "bucket": "*",
      "pattern": "^(.*)$",
      "template": "{{getModifyTimeFormatted .Entry \"2006/01/02\"}}/{{.Entry.Filename}}",
      "priority": 100
    }
  ]
}
```

**Result**: `2024/01/15/document.pdf`

### US Date Format

```json
{
  "rules": [
    {
      "bucket": "Archive-US",
      "pattern": "^(.*)$",
      "template": "{{getModifyTimeFormatted .Entry \"01/02/2006\"}}/{{.Entry.Filename}}",
      "priority": 200
    }
  ]
}
```

**Result**: `01/15/2024/document.pdf`

### Tag-Based Organization

```json
{
  "rules": [
    {
      "bucket": "Tagged-Data",
      "pattern": "^(.*)$",
      "template": "{{join (getTagsExplicit .Entry) \"/\"}}/{{.Entry.Filename}}",
      "priority": 300
    }
  ]
}
```

**Result**: `project-a/important/document.pdf`

### Size-Based Organization

```json
{
  "rules": [
    {
      "bucket": "Large-Files",
      "pattern": "^(.*)$",
      "template": "{{getSizeFormatted .Entry \"gb\"}}GB/{{.Entry.Filename}}",
      "priority": 400
    }
  ]
}
```

**Result**: `1GB/document.pdf`

### Complex Organization

```json
{
  "rules": [
    {
      "bucket": "Complex-Org",
      "pattern": "^(.*)$",
      "template": "{{.Entry.Volume}}/{{getModifyTimeFormatted .Entry \"2006\"}}/{{getModifyTimeFormatted .Entry \"01\"}}/{{.Entry.UID}}/{{getFilenameWithoutExt .Entry}}_{{getSizeFormatted .Entry \"mb\"}}MB{{getExtension .Entry}}",
      "priority": 500
    }
  ]
}
```

**Result**: `storage1/2024/01/1000/document_1MB.pdf`

### Conditional Organization

```json
{
  "rules": [
    {
      "bucket": "Size-Based",
      "pattern": "^(.*)$",
      "template": "{{if (gt .Entry.Size 1073741824)}}large{{else if (gt .Entry.Size 104857600)}}medium{{else}}small{{end}}/{{getModifyTimeFormatted .Entry \"2006/01\"}}/{{.Entry.Filename}}",
      "priority": 600
    }
  ]
}
```

**Result**: `large/2024/01/document.pdf`

## Rule Priority

Rules are applied in priority order (highest priority first). When a rule matches, it is applied and subsequent rules are ignored.

```json
{
  "rules": [
    {
      "bucket": "*",
      "pattern": "^(.*)$",
      "template": "default/{{.Filename}}",
      "priority": 100
    },
    {
      "bucket": "Special",
      "pattern": "^(.*)$",
      "template": "special/{{.Filename}}",
      "priority": 200
    }
  ]
}
```

In this example, files in the "Special" bucket will use the "special/" prefix (priority 200), while all other buckets will use the "default/" prefix (priority 100).

## Pattern Matching

The `pattern` field uses regular expressions to match object keys:

- `^(.*)$` - Match all objects
- `^data/(.*)$` - Match objects starting with "data/"
- `.*\.pdf$` - Match PDF files
- `^user-([0-9]+)/(.*)$` - Match user-specific paths

## Best Practices

1. **Use Descriptive Priorities**: Use priority values like 100, 200, 300 for easy management
2. **Test Templates**: Validate templates with sample data before deployment
3. **Handle Missing Data**: Use `default` function for optional fields
4. **Consider Performance**: Complex templates may impact performance
5. **Document Rules**: Add descriptions to rules for clarity

## Troubleshooting

### Common Issues

1. **Template Parse Errors**: Check template syntax and function names
2. **Missing Fields**: Ensure Starfish metadata includes required fields
3. **Priority Conflicts**: Verify rule priorities are set correctly
4. **Pattern Matching**: Test regex patterns with sample data

### Debugging

Enable debug logging to see template execution:

```bash
versitygw starfish --endpoint http://starfish-api:8080 --token your-token --path-rewrite-config rules.json --debug
```

### Validation

Validate your configuration file:

```bash
# Check JSON syntax
jq . rules.json

# Test with sample data
# (Add validation tools as needed)
```

## Migration from Other Systems

### AWS Object Lambda

If you're migrating from AWS Object Lambda, the template syntax is similar:

**Object Lambda**:
```javascript
return s3Object.key.replace(/^data\/(.*)/, 'archive/$1')
```

**Starfish Path Rewrite**:
```json
{
  "bucket": "*",
  "pattern": "^data/(.*)$",
  "template": "archive/{{index (split .OriginalKey \"/\") 1}}",
  "priority": 100
}
```

## Performance Considerations

- Template execution adds minimal overhead
- Complex templates with many functions may impact performance
- Consider caching for frequently accessed objects
- Monitor template execution time in high-throughput scenarios

## Security

- Template execution is sandboxed
- No arbitrary code execution
- Input validation prevents injection attacks
- Configuration files should have appropriate permissions 