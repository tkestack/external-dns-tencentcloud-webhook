/*
Copyright 2022 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package provider

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	log "github.com/sirupsen/logrus"
	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common"

	"sigs.k8s.io/external-dns/endpoint"
	"sigs.k8s.io/external-dns/plan"

	"github.com/tkestack/external-dns-tencentcloud-webhook/pkg/cloudapi"
)

const (
	TencentCloudEmptyPrefix = "@"
	DefaultAPIRate          = 9

	// ProviderSpecific property names.
	// Users set these via Service/Ingress annotations:
	//   external-dns.alpha.kubernetes.io/webhook-record-line       → "webhook/record-line"
	//   external-dns.alpha.kubernetes.io/webhook-record-line-id    → "webhook/record-line-id"
	//   external-dns.alpha.kubernetes.io/webhook-weight            → "webhook/weight"
	//   external-dns.alpha.kubernetes.io/webhook-mx                → "webhook/mx"
	//   external-dns.alpha.kubernetes.io/webhook-remark            → "webhook/remark"
	//   external-dns.alpha.kubernetes.io/webhook-status            → "webhook/status"
	PropertyRecordLine   = "webhook/record-line"
	PropertyRecordLineId = "webhook/record-line-id"
	PropertyWeight       = "webhook/weight"
	PropertyMX           = "webhook/mx"
	PropertyRemark       = "webhook/remark"
	PropertyStatus       = "webhook/status"
)

// getProviderSpecificString extracts a string property from endpoint's ProviderSpecific.
func getProviderSpecificString(ep *endpoint.Endpoint, key string) (string, bool) {
	return ep.GetProviderSpecificProperty(key)
}

// getProviderSpecificUint64 extracts a uint64 property from endpoint's ProviderSpecific.
func getProviderSpecificUint64(ep *endpoint.Endpoint, key string) (uint64, bool) {
	if v, ok := ep.GetProviderSpecificProperty(key); ok {
		if n, err := strconv.ParseUint(v, 10, 64); err == nil {
			return n, true
		}
	}
	return 0, false
}

// getProviderSpecificInt64 extracts an int64 property from endpoint's ProviderSpecific.
func getProviderSpecificInt64(ep *endpoint.Endpoint, key string) (int64, bool) {
	if v, ok := ep.GetProviderSpecificProperty(key); ok {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			return n, true
		}
	}
	return 0, false
}

// Config holds all configuration for the TencentCloud provider.
type Config struct {
	Region           string
	SecretId         string
	SecretKey        string
	VPCId            string
	ZoneType         string // "public" or "private"
	DomainFilter     []string
	ZoneIDFilter     []string
	DryRun           bool
	InternetEndpoint bool
	APIRate          int
	CredentialMode   string // "static" (default) or "oidc"
}

// ZoneIDFilter is a simple filter for zone IDs.
type ZoneIDFilter struct {
	ZoneIDs []string
}

func NewZoneIDFilter(zoneIDs []string) ZoneIDFilter {
	return ZoneIDFilter{ZoneIDs: zoneIDs}
}

func (f ZoneIDFilter) Match(zoneID string) bool {
	if len(f.ZoneIDs) == 0 {
		return true
	}
	for _, id := range f.ZoneIDs {
		if id == zoneID {
			return true
		}
	}
	return false
}

func (f ZoneIDFilter) IsConfigured() bool {
	return len(f.ZoneIDs) > 0
}

func NewTencentCloudProvider(cfg Config) (*TencentCloudProvider, error) {
	rate := cfg.APIRate
	if rate <= 0 {
		rate = DefaultAPIRate
	}

	// Build credential based on mode
	credential, err := buildCredential(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to build credential: %w", err)
	}

	var apiService cloudapi.TencentAPIService = cloudapi.NewTencentAPIService(cfg.Region, rate, credential, cfg.InternetEndpoint)
	if cfg.DryRun {
		apiService = cloudapi.NewReadOnlyAPIService(cfg.Region, rate, credential, cfg.InternetEndpoint)
	}

	domainFilter := endpoint.NewDomainFilter(cfg.DomainFilter)

	return &TencentCloudProvider{
		domainFilter: *domainFilter,
		zoneIDFilter: NewZoneIDFilter(cfg.ZoneIDFilter),
		apiService:   apiService,
		vpcID:        cfg.VPCId,
		privateZone:  cfg.ZoneType == "private",
	}, nil
}

// buildCredential creates the appropriate credential based on config mode.
func buildCredential(cfg Config) (common.CredentialIface, error) {
	switch cfg.CredentialMode {
	case "oidc":
		log.Info("Using OIDC credential mode (TKE RRSA)")
		provider, err := common.DefaultTkeOIDCRoleArnProvider()
		if err != nil {
			return nil, fmt.Errorf("failed to create OIDC provider: %w", err)
		}
		if cfg.Region != "" {
			provider.Endpoint = fmt.Sprintf("sts.%s.tencentcloudapi.com", cfg.Region)
		}
		cred, err := provider.GetCredential()
		if err != nil {
			return nil, fmt.Errorf("failed to get OIDC credential: %w", err)
		}
		return cred, nil
	default:
		// static mode: use SecretId/SecretKey
		if cfg.SecretId == "" || cfg.SecretKey == "" {
			return nil, fmt.Errorf("static credential mode requires SecretId and SecretKey")
		}
		log.Info("Using static credential mode (SecretId/SecretKey)")
		return common.NewCredential(cfg.SecretId, cfg.SecretKey), nil
	}
}

type TencentCloudProvider struct {
	apiService   cloudapi.TencentAPIService
	domainFilter endpoint.DomainFilter
	zoneIDFilter ZoneIDFilter
	vpcID        string // Private Zone only
	privateZone  bool
}

// GetDomainFilter returns the domain filter for webhook negotiation.
func (p *TencentCloudProvider) GetDomainFilter() endpoint.DomainFilter {
	return p.domainFilter
}

func (p *TencentCloudProvider) Records(ctx context.Context) ([]*endpoint.Endpoint, error) {
	if p.privateZone {
		return p.privateZoneRecords()
	}
	return p.dnsRecords()
}

func (p *TencentCloudProvider) ApplyChanges(ctx context.Context, changes *plan.Changes) error {
	if !changes.HasChanges() {
		return nil
	}

	log.Infof("apply changes. %s", cloudapi.JsonWrapper(changes))

	if p.privateZone {
		return p.applyChangesForPrivateZone(changes)
	}
	return p.applyChangesForDNS(changes)
}

// AdjustEndpoints normalizes desired endpoints to include default providerSpecific
// properties, ensuring stable comparison with Records() output.
func (p *TencentCloudProvider) AdjustEndpoints(endpoints []*endpoint.Endpoint) ([]*endpoint.Endpoint, error) {
	if !p.privateZone {
		// DNSPod (public zone) always returns record-line and status.
		// Only default record-line; record-line-id is a create-time-only hint
		// and must NOT be defaulted to "0" (which would override a custom RecordLine).
		for _, ep := range endpoints {
			if _, ok := getProviderSpecificString(ep, PropertyRecordLine); !ok {
				ep.SetProviderSpecificProperty(PropertyRecordLine, "默认")
			}
			if _, ok := getProviderSpecificString(ep, PropertyStatus); !ok {
				ep.SetProviderSpecificProperty(PropertyStatus, "ENABLE")
			}
			// Records() always sets SetIdentifier = Line.
			// Align desired endpoints: if user didn't set set-identifier,
			// default it to record-line value so both sides match.
			if ep.SetIdentifier == "" {
				if line, ok := getProviderSpecificString(ep, PropertyRecordLine); ok {
					ep.SetIdentifier = line
				}
			}
		}
	}
	return endpoints, nil
}

func getSubDomain(domain string, endpoint *endpoint.Endpoint) string {
	name := endpoint.DNSName
	name = name[:len(name)-len(domain)]
	name = strings.TrimSuffix(name, ".")

	if name == "" {
		return TencentCloudEmptyPrefix
	}
	return name
}

func getDnsDomain(subDomain string, domain string) string {
	if subDomain == TencentCloudEmptyPrefix {
		return domain
	}
	return subDomain + "." + domain
}
