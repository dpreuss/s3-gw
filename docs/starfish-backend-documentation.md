# Starfish Backend Documentation

## Introduction

This document provides comprehensive guidance on setting up, configuring, and utilizing the Starfish backend with VersityGW. The Starfish backend enables you to expose your Starfish data collections as S3-compatible buckets, allowing seamless access through standard S3 tools and applications.

## Prerequisites

Before you begin, ensure you have the following:

- **VersityGW Installation:** A working installation of VersityGW. Refer to the main VersityGW documentation for installation instructions.
- **Starfish API Endpoint:** The URL of your Starfish API endpoint.
- **Starfish Bearer Token:** A valid bearer token for authenticating with the Starfish API.
- **Starfish File Server (Optional but Recommended):** The URL of your Starfish file server for efficient GetObject operations.

## Setup and Installation

No separate installation steps are required for the Starfish backend beyond the standard VersityGW installation. The Starfish backend is integrated directly into the VersityGW binary.

## Configuration

The Starfish backend is configured via environment variables or command-line flags when starting VersityGW. Below are the key configuration parameters:

### Core Configuration

- **`VGW_BACKEND=starfish`**: Specifies the Starfish backend.
  - **Command Line Flag:** `--backend starfish`
  - **Environment Variable:** `VGW_BACKEND=starfish`

- **`VGW_BACKEND_ARG=<arg_string>`**: A string containing comma-separated key-value pairs for Starfish-specific configuration.

  The following parameters can be included in `VGW_BACKEND_ARG`:

  - **`api-endpoint=<url>` (Required)**:
    - The URL of the Starfish API endpoint (e.g., `https://starfish.example.com/api/v1`).
  - **`bearer-token=<token>` (Required)**:
    - The bearer token for authentication with the Starfish API.
  - **`file-server-url=<url>` (Optional)**:
    - The URL of the Starfish file server. This is used for GetObject operations to improve performance by directly streaming data.
  - **`cache-ttl=<duration>` (Optional)**:
    - The time-to-live for cached Starfish query results (e.g., `5m`, `1h`). Default is `1m`.
  - **`collections-refresh-interval=<duration>` (Optional)**:
    - The interval at which VersityGW refreshes the list of Starfish collections (e.g., `10m`, `1h`). Default is `10m`.
  - **`path-rewrite-config=<path>` (Optional)**:
    - Path to a JSON configuration file for path rewriting, allowing dynamic transformation of object paths based on metadata. Refer to `docs/path-rewrite.md` for more details.
  - **`tls-cert-file=<path>` (Optional)**:
    - Path to the TLS certificate file for client authentication when connecting to the Starfish API.
  - **`tls-key-file=<path>` (Optional)**:
    - Path to the TLS private key file for client authentication when connecting to the Starfish API.
  - **`tls-insecure-skip-verify=<bool>` (Optional)**:
    - Set to `true` to skip TLS certificate verification (use with caution, for testing only). Default is `false`.
  - **`tls-min-version=<version>` (Optional)**:
    - Minimum TLS version to use (e.g., `1.2`, `1.3`). Default is `1.2`.
  - **`connection-pool-size=<int>` (Optional)**:
    - The maximum number of idle HTTP connections to keep open in the pool for the Starfish API. Default is `100`.
  - **`max-idle-conns-per-host=<int>` (Optional)**:
    - The maximum number of idle HTTP connections per host for the Starfish API. Default is `10`.
  - **`idle-conn-timeout=<duration>` (Optional)**:
    - The amount of time an idle (keep-alive) connection will remain in the pool before closing. Default is `90s`.

### Example Configuration (`extra/example.conf` snippet)

```ini
[service]
; Other VersityGW configuration options...

[starfish_backend]
VGW_BACKEND="starfish"
VGW_BACKEND_ARG="\
  api-endpoint=https://starfish.example.com/api/v1,\
  bearer-token=your_starfish_api_token,\
  file-server-url=https://fileserver.starfish.example.com,\
  cache-ttl=5m,\
  collections-refresh-interval=15m,\
  path-rewrite-config=/etc/versitygw/path-rewrite.json,\
  tls-cert-file=/etc/versitygw/client.crt,\
  tls-key-file=/etc/versitygw/client.key,\
  tls-insecure-skip-verify=false,\
  tls-min-version=1.3,\
  connection-pool-size=200,\
  max-idle-conns-per-host=20,\
  idle-conn-timeout=120s"
```

## Usage Examples

Once VersityGW is configured with the Starfish backend, you can interact with your Starfish data collections using any S3-compatible client (e.g., AWS CLI, MinIO Client `mc`, S3 SDKs).

### 1. Listing Starfish Collections (Buckets)

Starfish collections tagged with `Collections:<tagset>` will appear as S3 buckets. For example, if you have a collection tagged `Collections:MyProjectData`, it will appear as an S3 bucket named `MyProjectData`.

**AWS CLI:**
```bash
aws s3 --endpoint-url http://localhost:7070 ls
```
*Expected Output (example):*
```
2023-10-26 10:00:00 MyProjectData
2023-10-26 10:05:00 AnotherCollection
```

### 2. Listing Objects within a Collection (Bucket)

To list objects within a Starfish collection, use the `ls` command with the bucket name.

**AWS CLI:**
```bash
aws s3 --endpoint-url http://localhost:7070 ls s3://MyProjectData/
```
*Expected Output (example):*
```
2023-10-26 10:10:00        12345 my_document.pdf
2023-10-26 10:12:00        56789 image.jpg
```

### 3. Downloading an Object

To download an object from a Starfish collection, use the `cp` command.

**AWS CLI:**
```bash
aws s3 --endpoint-url http://localhost:7070 cp s3://MyProjectData/my_document.pdf .
```

### 4. Uploading an Object (Note: Write operations are not yet supported for Starfish collections)

The Starfish S3 Collections Gateway currently provides read-only access to Starfish collections. Write operations (PutObject, DeleteObject, Multipart Uploads) are **not yet supported** in the current MVP. This feature will be considered in future releases.

### 5. Using Bucket Policies and ACLs

The Starfish backend integrates with VersityGW's existing bucket policy and ACL system. You can apply standard S3 bucket policies and ACLs to control access to your Starfish collections.

**Example: Granting Public Read Access to a Collection via Bucket Policy**

Create a file named `policy.json`:
```json
{
    "Version": "2012-10-17",
    "Statement": [
        {
            "Effect": "Allow",
            "Principal": "*",
            "Action": "s3:GetObject",
            "Resource": "arn:aws:s3:::MyProjectData/*"
        }
    ]
}
```
Apply the policy:
```bash
aws s3api --endpoint-url http://localhost:7070 put-bucket-policy --bucket MyProjectData --policy file://policy.json
```

### 6. Health Checks and Monitoring

VersityGW exposes Prometheus metrics and health check endpoints that include data from the Starfish backend.

- **Health Check:** `http://localhost:7070/health`
- **Metrics:** `http://localhost:7070/metrics`

Look for metrics prefixed with `starfish_` for Starfish-specific performance and caching statistics (e.g., `starfish_query_duration_seconds`, `starfish_cache_hit`, `starfish_cache_miss`).

## Advanced Topics

### Path Rewriting

The `path-rewrite-config` option allows you to define rules for transforming object paths as they are exposed through the S3 interface. This is useful for creating more user-friendly or standardized paths from complex Starfish internal paths. Refer to `docs/path-rewrite.md` and `extra/path-rewrite-example.json` for detailed examples.

### Error Handling

Errors from the Starfish API are translated into appropriate S3 API error responses, ensuring compatibility with S3 clients. For detailed error logs, refer to VersityGW's audit logs and backend logs.

## Future Considerations

- **Write Operations:** Support for PutObject, DeleteObject, and Multipart Uploads.
- **Object Tagging/Metadata:** Integration with Starfish object metadata for S3 object tags and custom metadata.
- **Event Notifications:** Support for S3 event notifications (e.g., S3:ObjectCreated) based on Starfish changes.
- **Performance Optimization:** Further enhancements like query batching for improved efficiency.

## Troubleshooting

- **Connection Refused:** Ensure the Starfish API endpoint and file server URL are correct and accessible from the VersityGW host.
- **Authentication Failed:** Verify the `bearer-token` is valid and has the necessary permissions in Starfish.
- **No Such Bucket/Key:** Confirm that the Starfish collection exists and is correctly tagged, and that the object key is valid.
- **Empty ListObjects Results:** Check if the Starfish collection is empty or if the configured `path-rewrite-config` is affecting visibility.

For more detailed troubleshooting, check the VersityGW logs for error messages related to the Starfish backend.
