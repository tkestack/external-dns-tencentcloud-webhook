package e2e_test

import (
	"fmt"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Record line (线路分组)", Label("record-line"), Ordered, func() {
	// Uses the existing external-dns-public release already running in the cluster.
	// No helm install/uninstall — this test only validates provider functionality.
	const release = "external-dns-public"

	AfterAll(func() {
		deleteTestService("e2e-line-dianxin")
		deleteTestService("e2e-line-liantong")
	})

	Context("single line", func() {
		It("should create record with custom line", func() {
			hostname := fmt.Sprintf("e2e-line.%s", domain)
			createTestServiceWithLine("e2e-line-dianxin", hostname, "电信")
			waitForServiceIP("e2e-line-dianxin", "default")

			// Wait for the record to be created with correct line
			Eventually(func() string {
				return getWebhookLogs(release)
			}, 3*time.Minute, 10*time.Second).Should(
				ContainSubstring("e2e-line"))

			logs := getWebhookLogs(release)
			Expect(logs).To(ContainSubstring("e2e-line"))
		})

		It("should be stable without update loop", func() {
			waitForStableSync(release)
		})
	})

	Context("multi-line with set-identifier", func() {
		It("should create records on different lines for same hostname", func() {
			hostname := fmt.Sprintf("e2e-line.%s", domain)
			createTestServiceWithLine("e2e-line-liantong", hostname, "联通")
			waitForServiceIP("e2e-line-liantong", "default")

			Eventually(func() bool {
				logs := getWebhookLogs(release)
				return strings.Contains(logs, `"RecordLine":"电信"`) &&
					strings.Contains(logs, `"RecordLine":"联通"`)
			}, 3*time.Minute, 10*time.Second).Should(BeTrue())
		})

		It("should be stable without update loop", func() {
			waitForStableSync(release)
		})

		It("should delete only matching line when service is removed", func() {
			deleteTestService("e2e-line-liantong")

			Eventually(func() string {
				return getWebhookLogs(release)
			}, 3*time.Minute, 10*time.Second).Should(ContainSubstring("DeleteRecord"))

			// The 电信 record should still exist (stable after delete)
			waitForStableSync(release)
		})
	})
})
