package provider

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common"
	dnspod "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/dnspod/v20210323"

	"sigs.k8s.io/external-dns/endpoint"
	"sigs.k8s.io/external-dns/plan"

	"github.com/tkestack/external-dns-tencentcloud-webhook/pkg/cloudapi"
)

// newTestProvider creates a TencentCloudProvider backed by a mock API service.
func newTestProvider(domains []*dnspod.DomainListItem, records map[string][]*dnspod.RecordListItem) *TencentCloudProvider {
	mockAPI := cloudapi.NewMockService(nil, nil, domains, records)
	return &TencentCloudProvider{
		apiService:   mockAPI,
		domainFilter: *endpoint.NewDomainFilter([]string{}),
		zoneIDFilter: NewZoneIDFilter(nil),
		privateZone:  false,
	}
}

func testDomain() *dnspod.DomainListItem {
	return &dnspod.DomainListItem{
		DomainId: common.Uint64Ptr(123),
		Name:     common.StringPtr("example.com"),
	}
}

// ---------------------------------------------------------------------------
// Test: Records() correctly reads back record-line from DNSPod
// ---------------------------------------------------------------------------

func TestRecords_DefaultLine(t *testing.T) {
	domains := []*dnspod.DomainListItem{testDomain()}
	records := map[string][]*dnspod.RecordListItem{
		"example.com": {
			{
				RecordId: common.Uint64Ptr(1),
				Name:     common.StringPtr("www"),
				Type:     common.StringPtr("A"),
				Value:    common.StringPtr("1.1.1.1"),
				TTL:      common.Uint64Ptr(600),
				Line:     common.StringPtr("默认"),
				LineId:   common.StringPtr("0"),
			},
		},
	}
	p := newTestProvider(domains, records)

	eps, err := p.Records(context.Background())
	require.NoError(t, err)
	require.Len(t, eps, 1)

	ep := eps[0]
	assert.Equal(t, "www.example.com", ep.DNSName)
	assert.Equal(t, "A", ep.RecordType)

	line, ok := ep.GetProviderSpecificProperty(PropertyRecordLine)
	assert.True(t, ok)
	assert.Equal(t, "默认", line)

	// record-line-id should NOT be in Records() output
	_, hasLineId := ep.GetProviderSpecificProperty(PropertyRecordLineId)
	assert.False(t, hasLineId, "record-line-id should not appear in Records() output")
}

func TestRecords_CustomLine(t *testing.T) {
	domains := []*dnspod.DomainListItem{testDomain()}
	records := map[string][]*dnspod.RecordListItem{
		"example.com": {
			{
				RecordId: common.Uint64Ptr(1),
				Name:     common.StringPtr("www"),
				Type:     common.StringPtr("A"),
				Value:    common.StringPtr("1.1.1.1"),
				TTL:      common.Uint64Ptr(600),
				Line:     common.StringPtr("电信"),
				LineId:   common.StringPtr("10=1"),
			},
		},
	}
	p := newTestProvider(domains, records)

	eps, err := p.Records(context.Background())
	require.NoError(t, err)
	require.Len(t, eps, 1)

	line, ok := eps[0].GetProviderSpecificProperty(PropertyRecordLine)
	assert.True(t, ok)
	assert.Equal(t, "电信", line)
}

// ---------------------------------------------------------------------------
// Test: groupDomainRecordList separates records by line
// ---------------------------------------------------------------------------

func TestGroupDomainRecordList_MultiLine(t *testing.T) {
	domains := []*dnspod.DomainListItem{testDomain()}
	records := map[string][]*dnspod.RecordListItem{
		"example.com": {
			{
				RecordId: common.Uint64Ptr(1),
				Name:     common.StringPtr("www"),
				Type:     common.StringPtr("A"),
				Value:    common.StringPtr("1.1.1.1"),
				TTL:      common.Uint64Ptr(600),
				Line:     common.StringPtr("默认"),
				LineId:   common.StringPtr("0"),
			},
			{
				RecordId: common.Uint64Ptr(2),
				Name:     common.StringPtr("www"),
				Type:     common.StringPtr("A"),
				Value:    common.StringPtr("2.2.2.2"),
				TTL:      common.Uint64Ptr(600),
				Line:     common.StringPtr("电信"),
				LineId:   common.StringPtr("10=1"),
			},
		},
	}
	p := newTestProvider(domains, records)

	eps, err := p.Records(context.Background())
	require.NoError(t, err)

	// Records on different lines should be separate endpoints
	assert.Len(t, eps, 2, "multi-line records should produce separate endpoints")

	// Verify each endpoint has 1 target and correct line
	lineMap := map[string]string{}
	for _, ep := range eps {
		line, _ := ep.GetProviderSpecificProperty(PropertyRecordLine)
		lineMap[line] = ep.Targets[0]
	}
	assert.Equal(t, "1.1.1.1", lineMap["默认"])
	assert.Equal(t, "2.2.2.2", lineMap["电信"])
}

// ---------------------------------------------------------------------------
// Test: SetIdentifier is populated from Line for multi-line records
// ---------------------------------------------------------------------------

func TestRecords_SetIdentifier_MultiLine(t *testing.T) {
	domains := []*dnspod.DomainListItem{testDomain()}
	records := map[string][]*dnspod.RecordListItem{
		"example.com": {
			{
				RecordId: common.Uint64Ptr(1),
				Name:     common.StringPtr("www"),
				Type:     common.StringPtr("A"),
				Value:    common.StringPtr("1.1.1.1"),
				TTL:      common.Uint64Ptr(600),
				Line:     common.StringPtr("电信"),
				LineId:   common.StringPtr("10=1"),
				Status:   common.StringPtr("ENABLE"),
			},
			{
				RecordId: common.Uint64Ptr(2),
				Name:     common.StringPtr("www"),
				Type:     common.StringPtr("A"),
				Value:    common.StringPtr("2.2.2.2"),
				TTL:      common.Uint64Ptr(600),
				Line:     common.StringPtr("联通"),
				LineId:   common.StringPtr("10=2"),
				Status:   common.StringPtr("ENABLE"),
			},
		},
	}
	p := newTestProvider(domains, records)

	eps, err := p.Records(context.Background())
	require.NoError(t, err)
	require.Len(t, eps, 2)

	// Multi-line: SetIdentifier should be populated with Line value
	sidMap := map[string]string{}
	for _, ep := range eps {
		sidMap[ep.SetIdentifier] = ep.Targets[0]
	}
	assert.Equal(t, "1.1.1.1", sidMap["电信"])
	assert.Equal(t, "2.2.2.2", sidMap["联通"])
}

func TestRecords_SetIdentifier_SingleLine(t *testing.T) {
	domains := []*dnspod.DomainListItem{testDomain()}
	records := map[string][]*dnspod.RecordListItem{
		"example.com": {
			{
				RecordId: common.Uint64Ptr(1),
				Name:     common.StringPtr("www"),
				Type:     common.StringPtr("A"),
				Value:    common.StringPtr("1.1.1.1"),
				TTL:      common.Uint64Ptr(600),
				Line:     common.StringPtr("电信"),
				LineId:   common.StringPtr("10=1"),
				Status:   common.StringPtr("ENABLE"),
			},
		},
	}
	p := newTestProvider(domains, records)

	eps, err := p.Records(context.Background())
	require.NoError(t, err)
	require.Len(t, eps, 1)

	// Single-line also gets SetIdentifier = Line for consistency
	assert.Equal(t, "电信", eps[0].SetIdentifier)
}

func TestRecords_SetIdentifier_MultiLine_WithTXT(t *testing.T) {
	// Simulate real scenario: A records + TXT ownership records on different lines
	domains := []*dnspod.DomainListItem{testDomain()}
	records := map[string][]*dnspod.RecordListItem{
		"example.com": {
			// A records
			{
				RecordId: common.Uint64Ptr(1),
				Name:     common.StringPtr("www"),
				Type:     common.StringPtr("A"),
				Value:    common.StringPtr("1.1.1.1"),
				TTL:      common.Uint64Ptr(600),
				Line:     common.StringPtr("电信"),
				LineId:   common.StringPtr("10=1"),
				Status:   common.StringPtr("ENABLE"),
			},
			{
				RecordId: common.Uint64Ptr(2),
				Name:     common.StringPtr("www"),
				Type:     common.StringPtr("A"),
				Value:    common.StringPtr("2.2.2.2"),
				TTL:      common.Uint64Ptr(600),
				Line:     common.StringPtr("联通"),
				LineId:   common.StringPtr("10=2"),
				Status:   common.StringPtr("ENABLE"),
			},
			// TXT ownership records (one per line)
			{
				RecordId: common.Uint64Ptr(3),
				Name:     common.StringPtr("a-www"),
				Type:     common.StringPtr("TXT"),
				Value:    common.StringPtr("\"heritage=external-dns,external-dns/owner=test\""),
				TTL:      common.Uint64Ptr(300),
				Line:     common.StringPtr("电信"),
				LineId:   common.StringPtr("10=1"),
				Status:   common.StringPtr("ENABLE"),
			},
			{
				RecordId: common.Uint64Ptr(4),
				Name:     common.StringPtr("a-www"),
				Type:     common.StringPtr("TXT"),
				Value:    common.StringPtr("\"heritage=external-dns,external-dns/owner=test\""),
				TTL:      common.Uint64Ptr(300),
				Line:     common.StringPtr("联通"),
				LineId:   common.StringPtr("10=2"),
				Status:   common.StringPtr("ENABLE"),
			},
		},
	}
	p := newTestProvider(domains, records)

	eps, err := p.Records(context.Background())
	require.NoError(t, err)

	// Should have 4 endpoints: 2 A (multi-line) + 2 TXT (multi-line)
	assert.Len(t, eps, 4)

	// Both A and TXT should have SetIdentifier set (since each type has multi-line)
	for _, ep := range eps {
		assert.NotEmpty(t, ep.SetIdentifier,
			"multi-line endpoint %s/%s should have SetIdentifier", ep.DNSName, ep.RecordType)
	}
}

// ---------------------------------------------------------------------------
// Test: AdjustEndpoints defaults SetIdentifier from record-line
// ---------------------------------------------------------------------------

func TestAdjustEndpoints_DefaultsSetIdentifier(t *testing.T) {
	p := &TencentCloudProvider{privateZone: false}

	// User didn't set set-identifier → AdjustEndpoints should default it to record-line
	ep := endpoint.NewEndpoint("www.example.com", "A", "1.1.1.1")
	// no SetIdentifier, no record-line → both get defaulted

	adjusted, err := p.AdjustEndpoints([]*endpoint.Endpoint{ep})
	require.NoError(t, err)

	assert.Equal(t, "默认", adjusted[0].SetIdentifier,
		"AdjustEndpoints should default SetIdentifier to record-line value")
}

func TestAdjustEndpoints_SetIdentifier_MatchesRecords(t *testing.T) {
	// Single-line, user set set-identifier=电信 → should match Records() output
	domains := []*dnspod.DomainListItem{testDomain()}
	records := map[string][]*dnspod.RecordListItem{
		"example.com": {
			{
				RecordId: common.Uint64Ptr(1),
				Name:     common.StringPtr("www"),
				Type:     common.StringPtr("A"),
				Value:    common.StringPtr("1.1.1.1"),
				TTL:      common.Uint64Ptr(600),
				Line:     common.StringPtr("电信"),
				LineId:   common.StringPtr("10=1"),
				Status:   common.StringPtr("ENABLE"),
			},
		},
	}
	p := newTestProvider(domains, records)

	current, err := p.Records(context.Background())
	require.NoError(t, err)

	// User sets set-identifier=电信 and record-line=电信
	desired := []*endpoint.Endpoint{
		endpoint.NewEndpoint("www.example.com", "A", "1.1.1.1"),
	}
	desired[0].SetIdentifier = "电信"
	desired[0].SetProviderSpecificProperty(PropertyRecordLine, "电信")

	adjusted, err := p.AdjustEndpoints(desired)
	require.NoError(t, err)

	// SetIdentifier and ProviderSpecific should all match
	assert.Equal(t, current[0].SetIdentifier, adjusted[0].SetIdentifier)
	assert.Equal(t, current[0].ProviderSpecific, adjusted[0].ProviderSpecific)
}

// ---------------------------------------------------------------------------
// Test: AdjustEndpoints defaults record-line but NOT record-line-id
// ---------------------------------------------------------------------------

func TestAdjustEndpoints_DefaultsRecordLine(t *testing.T) {
	p := &TencentCloudProvider{privateZone: false}

	eps := []*endpoint.Endpoint{
		endpoint.NewEndpoint("www.example.com", "A", "1.1.1.1"),
	}

	adjusted, err := p.AdjustEndpoints(eps)
	require.NoError(t, err)
	require.Len(t, adjusted, 1)

	line, ok := adjusted[0].GetProviderSpecificProperty(PropertyRecordLine)
	assert.True(t, ok)
	assert.Equal(t, "默认", line)

	// Must NOT default record-line-id (was the root cause of the line bug)
	_, hasLineId := adjusted[0].GetProviderSpecificProperty(PropertyRecordLineId)
	assert.False(t, hasLineId, "AdjustEndpoints must NOT default record-line-id")
}

func TestAdjustEndpoints_PreservesCustomLine(t *testing.T) {
	p := &TencentCloudProvider{privateZone: false}

	ep := endpoint.NewEndpoint("www.example.com", "A", "1.1.1.1")
	ep.SetProviderSpecificProperty(PropertyRecordLine, "电信")

	adjusted, err := p.AdjustEndpoints([]*endpoint.Endpoint{ep})
	require.NoError(t, err)

	line, _ := adjusted[0].GetProviderSpecificProperty(PropertyRecordLine)
	assert.Equal(t, "电信", line)
}

// ---------------------------------------------------------------------------
// Test: CreateRecord sends only RecordLine (not RecordLineId) when user
// specifies record-line via annotation
// ---------------------------------------------------------------------------

func TestCreateRecord_OnlyRecordLine(t *testing.T) {
	domains := []*dnspod.DomainListItem{testDomain()}
	records := map[string][]*dnspod.RecordListItem{
		"example.com": {},
	}
	p := newTestProvider(domains, records)

	ep := endpoint.NewEndpoint("www.example.com", "A", "1.1.1.1")
	ep.SetProviderSpecificProperty(PropertyRecordLine, "电信")

	changes := &plan.Changes{
		Create: []*endpoint.Endpoint{ep},
	}

	err := p.ApplyChanges(context.Background(), changes)
	require.NoError(t, err)

	// Verify the record was created with correct line
	eps, err := p.Records(context.Background())
	require.NoError(t, err)
	require.Len(t, eps, 1)

	line, ok := eps[0].GetProviderSpecificProperty(PropertyRecordLine)
	assert.True(t, ok)
	assert.Equal(t, "电信", line)
}

func TestCreateRecord_WithRecordLineId(t *testing.T) {
	domains := []*dnspod.DomainListItem{testDomain()}
	records := map[string][]*dnspod.RecordListItem{
		"example.com": {},
	}
	p := newTestProvider(domains, records)

	ep := endpoint.NewEndpoint("www.example.com", "A", "1.1.1.1")
	ep.SetProviderSpecificProperty(PropertyRecordLineId, "10=1")

	changes := &plan.Changes{
		Create: []*endpoint.Endpoint{ep},
	}

	err := p.ApplyChanges(context.Background(), changes)
	require.NoError(t, err)

	eps, err := p.Records(context.Background())
	require.NoError(t, err)
	require.Len(t, eps, 1)
	// Record should exist (LineId was used for creation)
	assert.Equal(t, "1.1.1.1", eps[0].Targets[0])
}

// ---------------------------------------------------------------------------
// Test: No spurious update cycle when record-line matches
// ---------------------------------------------------------------------------

func TestNoSpuriousUpdate_LineMatches(t *testing.T) {
	domains := []*dnspod.DomainListItem{testDomain()}
	records := map[string][]*dnspod.RecordListItem{
		"example.com": {
			{
				RecordId: common.Uint64Ptr(1),
				Name:     common.StringPtr("www"),
				Type:     common.StringPtr("A"),
				Value:    common.StringPtr("1.1.1.1"),
				TTL:      common.Uint64Ptr(600),
				Line:     common.StringPtr("电信"),
				LineId:   common.StringPtr("10=1"),
				Status:   common.StringPtr("ENABLE"),
			},
		},
	}
	p := newTestProvider(domains, records)

	// Simulate what external-dns does: Records() → AdjustEndpoints(desired) → diff
	current, err := p.Records(context.Background())
	require.NoError(t, err)

	desired := []*endpoint.Endpoint{
		endpoint.NewEndpoint("www.example.com", "A", "1.1.1.1"),
	}
	desired[0].SetProviderSpecificProperty(PropertyRecordLine, "电信")

	adjusted, err := p.AdjustEndpoints(desired)
	require.NoError(t, err)

	// Compare providerSpecific: should be identical → no update needed
	assert.Equal(t, current[0].ProviderSpecific, adjusted[0].ProviderSpecific,
		"ProviderSpecific should match between Records() and AdjustEndpoints(), no spurious update")
}

func TestSpuriousUpdate_WhenLineIdWasDefaulted(t *testing.T) {
	// This test verifies the OLD bug is fixed:
	// Previously AdjustEndpoints added record-line-id=0 to desired,
	// but Records() also returned record-line-id from DNSPod.
	// When user set record-line=电信, the record-line-id=0 in desired
	// caused API to create with "默认" line, leading to infinite update.

	domains := []*dnspod.DomainListItem{testDomain()}
	records := map[string][]*dnspod.RecordListItem{
		"example.com": {
			{
				RecordId: common.Uint64Ptr(1),
				Name:     common.StringPtr("www"),
				Type:     common.StringPtr("A"),
				Value:    common.StringPtr("1.1.1.1"),
				TTL:      common.Uint64Ptr(600),
				Line:     common.StringPtr("电信"),
				LineId:   common.StringPtr("10=1"),
				Status:   common.StringPtr("ENABLE"),
			},
		},
	}
	p := newTestProvider(domains, records)

	current, err := p.Records(context.Background())
	require.NoError(t, err)

	// User sets record-line=电信 via annotation (no record-line-id)
	desired := []*endpoint.Endpoint{
		endpoint.NewEndpoint("www.example.com", "A", "1.1.1.1"),
	}
	desired[0].SetProviderSpecificProperty(PropertyRecordLine, "电信")

	adjusted, err := p.AdjustEndpoints(desired)
	require.NoError(t, err)

	// After fix: no record-line-id in either side → match
	assert.Equal(t, current[0].ProviderSpecific, adjusted[0].ProviderSpecific,
		"No spurious diff: record-line-id is not in either Records() or AdjustEndpoints()")
}

// ---------------------------------------------------------------------------
// Test: Delete does not affect records on other lines
// ---------------------------------------------------------------------------

func TestDelete_ShouldNotAffectOtherLines(t *testing.T) {
	domains := []*dnspod.DomainListItem{testDomain()}
	records := map[string][]*dnspod.RecordListItem{
		"example.com": {
			// external-dns managed record (默认 line)
			{
				RecordId: common.Uint64Ptr(1),
				Name:     common.StringPtr("www"),
				Type:     common.StringPtr("A"),
				Value:    common.StringPtr("1.1.1.1"),
				TTL:      common.Uint64Ptr(600),
				Line:     common.StringPtr("默认"),
				LineId:   common.StringPtr("0"),
			},
			// manually created record (封禁线路), same value
			{
				RecordId: common.Uint64Ptr(2),
				Name:     common.StringPtr("www"),
				Type:     common.StringPtr("A"),
				Value:    common.StringPtr("1.1.1.1"),
				TTL:      common.Uint64Ptr(600),
				Line:     common.StringPtr("境外封禁"),
				LineId:   common.StringPtr("99=1"),
			},
		},
	}
	p := newTestProvider(domains, records)

	// Delete the "默认" line record
	ep := endpoint.NewEndpoint("www.example.com", "A", "1.1.1.1")
	ep.SetProviderSpecificProperty(PropertyRecordLine, "默认")

	changes := &plan.Changes{
		Delete: []*endpoint.Endpoint{ep},
	}

	err := p.ApplyChanges(context.Background(), changes)
	require.NoError(t, err)

	// Check remaining records — only the "境外封禁" record should survive
	eps, err := p.Records(context.Background())
	require.NoError(t, err)
	assert.Len(t, eps, 1, "Delete should only remove records matching the same line")

	line, ok := eps[0].GetProviderSpecificProperty(PropertyRecordLine)
	assert.True(t, ok)
	assert.Equal(t, "境外封禁", line)
}

// ---------------------------------------------------------------------------
// Test: Weight=0 should not produce spurious diff
// ---------------------------------------------------------------------------

func TestRecords_WeightZero_NotIncluded(t *testing.T) {
	domains := []*dnspod.DomainListItem{testDomain()}
	weight := uint64(0)
	records := map[string][]*dnspod.RecordListItem{
		"example.com": {
			{
				RecordId: common.Uint64Ptr(1),
				Name:     common.StringPtr("www"),
				Type:     common.StringPtr("A"),
				Value:    common.StringPtr("1.1.1.1"),
				TTL:      common.Uint64Ptr(600),
				Line:     common.StringPtr("默认"),
				LineId:   common.StringPtr("0"),
				Weight:   &weight,
			},
		},
	}
	p := newTestProvider(domains, records)

	eps, err := p.Records(context.Background())
	require.NoError(t, err)
	require.Len(t, eps, 1)

	// Weight=0 means "disabled", should NOT appear in providerSpecific
	_, hasWeight := eps[0].GetProviderSpecificProperty(PropertyWeight)
	assert.False(t, hasWeight, "Weight=0 should not be in providerSpecific")
}

// ---------------------------------------------------------------------------
// Test: Status field round-trip
// ---------------------------------------------------------------------------

func TestRecords_Status_Included(t *testing.T) {
	domains := []*dnspod.DomainListItem{testDomain()}
	records := map[string][]*dnspod.RecordListItem{
		"example.com": {
			{
				RecordId: common.Uint64Ptr(1),
				Name:     common.StringPtr("www"),
				Type:     common.StringPtr("A"),
				Value:    common.StringPtr("1.1.1.1"),
				TTL:      common.Uint64Ptr(600),
				Line:     common.StringPtr("默认"),
				LineId:   common.StringPtr("0"),
				Status:   common.StringPtr("ENABLE"),
			},
		},
	}
	p := newTestProvider(domains, records)

	eps, err := p.Records(context.Background())
	require.NoError(t, err)
	require.Len(t, eps, 1)

	// Status should be included in Records() for round-trip consistency
	status, hasStatus := eps[0].GetProviderSpecificProperty(PropertyStatus)
	assert.True(t, hasStatus, "Status should be included in Records() output")
	assert.Equal(t, "ENABLE", status)
}

func TestRecords_Status_Disabled(t *testing.T) {
	domains := []*dnspod.DomainListItem{testDomain()}
	records := map[string][]*dnspod.RecordListItem{
		"example.com": {
			{
				RecordId: common.Uint64Ptr(1),
				Name:     common.StringPtr("www"),
				Type:     common.StringPtr("A"),
				Value:    common.StringPtr("1.1.1.1"),
				TTL:      common.Uint64Ptr(600),
				Line:     common.StringPtr("默认"),
				LineId:   common.StringPtr("0"),
				Status:   common.StringPtr("DISABLE"),
			},
		},
	}
	p := newTestProvider(domains, records)

	eps, err := p.Records(context.Background())
	require.NoError(t, err)
	require.Len(t, eps, 1)

	status, hasStatus := eps[0].GetProviderSpecificProperty(PropertyStatus)
	assert.True(t, hasStatus)
	assert.Equal(t, "DISABLE", status)
}
