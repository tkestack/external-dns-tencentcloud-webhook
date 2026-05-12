package e2e_test

import (
	"flag"
	"fmt"
	"os/exec"
	"strings"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var (
	domain        string
	privateDomain string
	clusterID     string
	imageTag      string
)

func init() {
	flag.StringVar(&domain, "domain", "tet.com", "Public domain for DNS tests")
	flag.StringVar(&privateDomain, "private-domain", "", "Private zone domain (default: cls-<cluster-id>.local)")
	flag.StringVar(&clusterID, "cluster-id", "", "Cluster ID (auto-detected if empty)")
	flag.StringVar(&imageTag, "image-tag", "", "Webhook image tag override (e.g. latest)")
}

func TestE2E(t *testing.T) {
	RegisterFailHandler(Fail)

	// Skip if kubectl is not connected
	if err := exec.Command("kubectl", "cluster-info").Run(); err != nil {
		t.Skip("kubectl not connected to a cluster, skipping e2e tests")
	}

	RunSpecs(t, "external-dns-tencentcloud-webhook E2E Suite")
}

var _ = BeforeSuite(func() {
	// Auto-detect cluster ID
	if clusterID == "" {
		out, err := exec.Command("bash", "-c",
			`kubectl cluster-info 2>/dev/null | head -1 | grep -oE "cls-[a-z0-9]+"`).Output()
		Expect(err).NotTo(HaveOccurred(), "failed to detect cluster ID")
		clusterID = strings.TrimSpace(string(out))
	}

	// Detect region
	region := ""
	out, err := exec.Command("bash", "-c",
		`kubectl get --raw /.well-known/openid-configuration 2>/dev/null | python3 -c "import json,sys;print(json.load(sys.stdin)['issuer'].split('//')[1].split('-oidc')[0])" 2>/dev/null`).Output()
	if err == nil {
		region = strings.TrimSpace(string(out))
	}

	// Detect context name
	context := ""
	out, err = exec.Command("kubectl", "config", "current-context").Output()
	if err == nil {
		context = strings.TrimSpace(string(out))
	}

	// Auto-detect private domain from cluster ID
	if privateDomain == "" {
		privateDomain = fmt.Sprintf("%s.local", clusterID)
	}

	// Print test target
	fmt.Fprintf(GinkgoWriter, "\n")
	fmt.Fprintf(GinkgoWriter, "════════════════════════════════════════\n")
	fmt.Fprintf(GinkgoWriter, "  E2E Test Target\n")
	fmt.Fprintf(GinkgoWriter, "  Cluster:  %s\n", clusterID)
	fmt.Fprintf(GinkgoWriter, "  Region:   %s\n", region)
	fmt.Fprintf(GinkgoWriter, "  Context:  %s\n", context)
	fmt.Fprintf(GinkgoWriter, "  Domain:   %s (public)\n", domain)
	fmt.Fprintf(GinkgoWriter, "  Private:  %s\n", privateDomain)
	fmt.Fprintf(GinkgoWriter, "  Image:    %s\n", imageTag)
	fmt.Fprintf(GinkgoWriter, "════════════════════════════════════════\n")
	fmt.Fprintf(GinkgoWriter, "\n")
})
