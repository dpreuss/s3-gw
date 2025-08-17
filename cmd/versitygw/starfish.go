// Copyright 2025 Starfish Storage
// This file is licensed under the Apache License, Version 2.0
// (the "License"); you may not use this file except in compliance
// with the License.  You may obtain a copy of the License at
//
//   http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.

package main

import (
	"context"
	"fmt"
	"time"

	"github.com/urfave/cli/v2"
	"github.com/versity/versitygw/backend/starfish"
)

var (
	starfishAPIEndpoint                string
	starfishBearerToken                string
	starfishFileServerURL              string
	starfishCacheTTL                   int
	starfishCollectionsRefreshInterval int
	starfishPathRewriteConfig          string

	// TLS Configuration
	starfishTLSCertFile           string
	starfishTLSKeyFile            string
	starfishTLSInsecureSkipVerify bool
	starfishTLSMinVersion         string

	// Performance and Monitoring
	starfishConnectionPoolSize  int
	starfishMaxIdleConnsPerHost int
	starfishIdleConnTimeout     time.Duration
)

func starfishCommand() *cli.Command {
	return &cli.Command{
		Name:  "starfish",
		Usage: "starfish metadata search backend",
		Description: `Starfish is a metadata search system that provides fast querying of file
metadata across large storage systems. This backend allows S3 clients to
browse and search files indexed by Starfish as if they were S3 objects.

Note: This is a read-only backend. Write operations (PutObject, DeleteObject,
etc.) are not supported.

Example usage:
versitygw starfish --endpoint http://starfish-api:8080 --token your-bearer-token`,
		Action: runStarfish,
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:        "endpoint",
				Usage:       "starfish API endpoint URL",
				EnvVars:     []string{"VGW_STARFISH_ENDPOINT"},
				Destination: &starfishAPIEndpoint,
				Required:    true,
			},
			&cli.StringFlag{
				Name:        "token",
				Usage:       "starfish API bearer token (also used for file server authentication)",
				EnvVars:     []string{"VGW_STARFISH_TOKEN"},
				Destination: &starfishBearerToken,
				Required:    true,
			},
			&cli.StringFlag{
				Name:        "file-server",
				Usage:       "starfish file server URL for GetObject operations (optional)",
				EnvVars:     []string{"VGW_STARFISH_FILE_SERVER"},
				Destination: &starfishFileServerURL,
			},
			&cli.IntFlag{
				Name:        "cache-ttl",
				Usage:       "cache TTL in minutes for starfish query results",
				EnvVars:     []string{"VGW_STARFISH_CACHE_TTL"},
				Destination: &starfishCacheTTL,
				Value:       60,
			},
			&cli.IntFlag{
				Name:        "collections-refresh-interval",
				Usage:       "interval in minutes to refresh Starfish collections (default: 10)",
				EnvVars:     []string{"VGW_STARFISH_COLLECTIONS_REFRESH_INTERVAL"},
				Destination: &starfishCollectionsRefreshInterval,
				Value:       10,
			},
			&cli.StringFlag{
				Name:        "path-rewrite-config",
				Usage:       "path to path rewrite rules configuration file (optional)",
				EnvVars:     []string{"VGW_STARFISH_PATH_REWRITE_CONFIG"},
				Destination: &starfishPathRewriteConfig,
			},
			&cli.StringFlag{
				Name:        "tls-cert",
				Usage:       "path to TLS certificate file for Starfish API connections",
				EnvVars:     []string{"VGW_STARFISH_TLS_CERT"},
				Destination: &starfishTLSCertFile,
			},
			&cli.StringFlag{
				Name:        "tls-key",
				Usage:       "path to TLS private key file for Starfish API connections",
				EnvVars:     []string{"VGW_STARFISH_TLS_KEY"},
				Destination: &starfishTLSKeyFile,
			},
			&cli.BoolFlag{
				Name:        "tls-insecure",
				Usage:       "skip TLS certificate verification (for testing only)",
				EnvVars:     []string{"VGW_STARFISH_TLS_INSECURE"},
				Destination: &starfishTLSInsecureSkipVerify,
			},
			&cli.StringFlag{
				Name:        "tls-min-version",
				Usage:       "minimum TLS version (1.2 or 1.3, default: 1.2)",
				EnvVars:     []string{"VGW_STARFISH_TLS_MIN_VERSION"},
				Destination: &starfishTLSMinVersion,
				Value:       "1.2",
			},
			&cli.IntFlag{
				Name:        "connection-pool-size",
				Usage:       "HTTP connection pool size (default: 100)",
				EnvVars:     []string{"VGW_STARFISH_CONNECTION_POOL_SIZE"},
				Destination: &starfishConnectionPoolSize,
				Value:       100,
			},
			&cli.IntFlag{
				Name:        "max-idle-conns-per-host",
				Usage:       "maximum idle connections per host (default: 10)",
				EnvVars:     []string{"VGW_STARFISH_MAX_IDLE_CONNS_PER_HOST"},
				Destination: &starfishMaxIdleConnsPerHost,
				Value:       10,
			},
			&cli.DurationFlag{
				Name:        "idle-conn-timeout",
				Usage:       "idle connection timeout (default: 90s)",
				EnvVars:     []string{"VGW_STARFISH_IDLE_CONN_TIMEOUT"},
				Destination: &starfishIdleConnTimeout,
				Value:       90 * time.Second,
			},
		},
	}
}

func runStarfish(ctx *cli.Context) error {
	if starfishAPIEndpoint == "" {
		return fmt.Errorf("starfish API endpoint is required")
	}

	if starfishBearerToken == "" {
		return fmt.Errorf("starfish bearer token is required")
	}

	// Load path rewrite configuration if specified
	var pathRewriteConfig *starfish.PathRewriteConfig
	if starfishPathRewriteConfig != "" {
		var err error
		pathRewriteConfig, err = starfish.LoadPathRewriteConfig(starfishPathRewriteConfig)
		if err != nil {
			return fmt.Errorf("failed to load path rewrite configuration: %w", err)
		}
		fmt.Printf("Loaded path rewrite configuration from: %s\n", starfishPathRewriteConfig)
	}

	config := &starfish.StarfishConfig{
		APIEndpoint:                starfishAPIEndpoint,
		BearerToken:                starfishBearerToken,
		FileServerURL:              starfishFileServerURL,
		CacheTTL:                   time.Duration(starfishCacheTTL) * time.Minute,
		CollectionsRefreshInterval: time.Duration(starfishCollectionsRefreshInterval) * time.Minute,
		PathRewriteConfig:          pathRewriteConfig,
		TLSCertFile:                starfishTLSCertFile,
		TLSKeyFile:                 starfishTLSKeyFile,
		TLSInsecureSkipVerify:      starfishTLSInsecureSkipVerify,
		TLSMinVersion:              starfishTLSMinVersion,
		ConnectionPoolSize:         starfishConnectionPoolSize,
		MaxIdleConnsPerHost:        starfishMaxIdleConnsPerHost,
		IdleConnTimeout:            starfishIdleConnTimeout,
	}

	be, err := starfish.NewStarfishBackend(config)
	if err != nil {
		return fmt.Errorf("failed to init starfish backend: %w", err)
	}

	// Initialize collections by discovering Collections: tagset tags
	fmt.Println("Initializing Starfish collections...")
	if err := be.InitializeCollections(ctx.Context); err != nil {
		return fmt.Errorf("failed to initialize collections: %w", err)
	}

	// Start background refresh goroutine
	go func(be *starfish.StarfishBackend, ctx context.Context) {
		interval := be.CollectionsRefreshInterval
		if interval <= 0 {
			interval = 10 * time.Minute
		}
		for {
			select {
			case <-ctx.Done():
				return
			case <-time.After(interval):
				fmt.Println("Refreshing Starfish collections...")
				if err := be.InitializeCollections(ctx); err != nil {
					fmt.Printf("[WARN] Failed to refresh Starfish collections: %v\n", err)
				}
			}
		}
	}(be, ctx.Context)

	// Report discovered collections
	collections := be.GetAllCollections()
	if len(collections) == 0 {
		fmt.Println("No tags found in Collections: tagset - no S3 buckets will be available")
	} else {
		fmt.Printf("Discovered %d collections from Collections: tagset:\n", len(collections))
		for bucketName, collectionTag := range collections {
			fmt.Printf("  - Bucket: %s -> Tag: %s\n", bucketName, collectionTag)
		}
	}

	return runGateway(ctx.Context, be)
}
