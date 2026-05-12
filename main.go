package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"runtime"
	"strings"

	log "github.com/sirupsen/logrus"

	"github.com/tkestack/external-dns-tencentcloud-webhook/pkg/provider"
	"github.com/tkestack/external-dns-tencentcloud-webhook/pkg/server"
)

// Build-time variables injected via ldflags.
var (
	version = "dev"
	commit  = "unknown"
	date    = "unknown"
)

// arrayFlags allows repeated --flag=value usage.
type arrayFlags []string

func (a *arrayFlags) String() string { return strings.Join(*a, ",") }
func (a *arrayFlags) Set(value string) error {
	*a = append(*a, value)
	return nil
}

// configFile represents the JSON config file format, compatible with the former
// in-tree tencentcloud provider (--tencent-cloud-config-file).
type configFile struct {
	RegionId         string `json:"regionId"`
	SecretId         string `json:"secretId"`
	SecretKey        string `json:"secretKey"`
	VPCId            string `json:"vpcId"`
	InternetEndpoint *bool  `json:"internetEndpoint"`
}

func main() {
	var (
		cfgFile          string
		credentialMode   string
		region           string
		secretId         string
		secretKey        string
		vpcId            string
		zoneType         string
		dryRun           bool
		internetEndpoint bool
		apiRate          int
		providerPort     string
		healthPort       string
		logLevel         string
		domainFilter     arrayFlags
		zoneIDFilter     arrayFlags
	)

	flag.StringVar(&cfgFile, "config-file", "", "JSON config file path (compatible with in-tree --tencent-cloud-config-file)")
	flag.StringVar(&credentialMode, "credential-mode", "static", "Credential mode: static (SecretId/SecretKey) or oidc (TKE RRSA)")
	flag.StringVar(&region, "region", "", "Tencent Cloud region (env: TENCENTCLOUD_REGION)")
	flag.StringVar(&secretId, "secret-id", "", "Tencent Cloud SecretId (env: TENCENTCLOUD_SECRET_ID)")
	flag.StringVar(&secretKey, "secret-key", "", "Tencent Cloud SecretKey (env: TENCENTCLOUD_SECRET_KEY)")
	flag.StringVar(&vpcId, "vpc-id", "", "VPC ID for PrivateDNS")
	flag.StringVar(&zoneType, "zone-type", "public", "Zone type: public or private")
	flag.BoolVar(&dryRun, "dry-run", false, "Dry run mode, no changes will be made")
	flag.BoolVar(&internetEndpoint, "internet-endpoint", true, "Use internet endpoint (false for internal endpoint)")
	flag.IntVar(&apiRate, "api-rate", 9, "API rate limit per second")
	flag.StringVar(&providerPort, "provider-port", "localhost:8888", "Provider HTTP server address")
	flag.StringVar(&healthPort, "health-port", ":8080", "Health check HTTP server address")
	flag.StringVar(&logLevel, "log-level", "info", "Log level (debug, info, warn, error)")
	flag.Var(&domainFilter, "domain-filter", "Domain filter (can be specified multiple times)")
	flag.Var(&zoneIDFilter, "zone-id-filter", "Zone ID filter (can be specified multiple times)")
	flag.Parse()

	// Configure logging
	ll, err := log.ParseLevel(logLevel)
	if err != nil {
		log.Fatalf("Invalid log level: %v", err)
	}
	log.SetLevel(ll)

	log.Infof("external-dns-tencentcloud-webhook version=%s commit=%s date=%s go=%s platform=%s/%s",
		version, commit, date, runtime.Version(), runtime.GOOS, runtime.GOARCH)

	// Load config file if specified (lowest priority, overridden by flags/env)
	if cfgFile != "" {
		fileCfg, err := loadConfigFile(cfgFile)
		if err != nil {
			log.Fatalf("Failed to load config file: %v", err)
		}
		// Config file provides defaults; flags and env vars override
		if region == "" {
			region = fileCfg.RegionId
		}
		if secretId == "" {
			secretId = fileCfg.SecretId
		}
		if secretKey == "" {
			secretKey = fileCfg.SecretKey
		}
		if vpcId == "" {
			vpcId = fileCfg.VPCId
		}
		if fileCfg.InternetEndpoint != nil && !isFlagSet("internet-endpoint") {
			internetEndpoint = *fileCfg.InternetEndpoint
		}
		log.Infof("Loaded config file: %s", cfgFile)
	}

	// Environment variables override config file but not explicit flags
	if secretId == "" {
		secretId = os.Getenv("TENCENTCLOUD_SECRET_ID")
	}
	if secretKey == "" {
		secretKey = os.Getenv("TENCENTCLOUD_SECRET_KEY")
	}
	if region == "" {
		region = os.Getenv("TENCENTCLOUD_REGION")
	}
	if region == "" {
		// TKE_REGION is injected by pod-identity-webhook when SA has
		// tke.cloud.tencent.com/role-arn annotation (OIDC mode)
		region = os.Getenv("TKE_REGION")
	}

	// Static mode: credentials required from flags/env/configFile
	if credentialMode != "oidc" {
		if secretId == "" || secretKey == "" {
			log.Fatal("Tencent Cloud credentials required. Provide via:\n" +
				"  1. --config-file (JSON file with secretId/secretKey)\n" +
				"  2. --secret-id / --secret-key flags\n" +
				"  3. TENCENTCLOUD_SECRET_ID / TENCENTCLOUD_SECRET_KEY env vars\n" +
				"  4. --credential-mode=oidc (for TKE RRSA, no static keys needed)")
		}
	}

	cfg := provider.Config{
		Region:           region,
		SecretId:         secretId,
		SecretKey:        secretKey,
		VPCId:            vpcId,
		ZoneType:         zoneType,
		DomainFilter:     domainFilter,
		ZoneIDFilter:     zoneIDFilter,
		DryRun:           dryRun,
		InternetEndpoint: internetEndpoint,
		APIRate:          apiRate,
		CredentialMode:   credentialMode,
	}

	p, err := provider.NewTencentCloudProvider(cfg)
	if err != nil {
		log.Fatalf("Failed to create provider: %v", err)
	}

	log.Infof("Starting external-dns-tencentcloud-webhook (region=%s, zone-type=%s, dry-run=%v)", region, zoneType, dryRun)

	// Start health server in background
	go func() {
		if err := server.StartHealthServer(healthPort); err != nil {
			log.Fatalf("Health server failed: %v", err)
		}
	}()

	// Start provider server (blocking)
	if err := server.StartProviderServer(p, providerPort); err != nil {
		log.Fatalf("Provider server failed: %v", err)
	}
}

// loadConfigFile reads and parses the JSON config file.
func loadConfigFile(path string) (*configFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	var cfg configFile
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	return &cfg, nil
}

// isFlagSet checks if a flag was explicitly set on the command line.
func isFlagSet(name string) bool {
	found := false
	flag.Visit(func(f *flag.Flag) {
		if f.Name == name {
			found = true
		}
	})
	return found
}
