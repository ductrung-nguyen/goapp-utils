package utils

import (
	"fmt"
	"os"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/util/uuid"
)

const sampleKubeConfig = `
apiVersion: v1
clusters:
- cluster:
    insecure-skip-tls-verify: true
    server: https://127.0.0.1:6443
  name: test-cluster
contexts:
- context:
    cluster: test-cluster
    namespace: test-namespace
    user: test-user
  name: test-namespace/test-cluster/test-user
current-context: test-namespace/test-cluster/test-user
kind: Config
preferences: {}
users:
- name: test-user
  user:
    token: sha256~synthetic-test-token
`

var _ = Describe("Test Kubernetes client", func() {
	Context("Out-of-cluster", func() {
		Context("Using wrong config path", func() {
			It("should raise error", func() {
				_, err := K8sClientHelper{}.GetClient("/tmp/path" + fmt.Sprint(uuid.NewUUID()))
				Expect(err).Should(HaveOccurred())
			})
		})

		Context("Using good config path", func() {
			It("should load OK", func() {
				f, err := os.CreateTemp(".", "*.yaml")
				if err != nil {
					panic(err)
				}
				defer func() {
					if err := f.Close(); err != nil {
						panic(err)
					}
				}()
				filename := f.Name()
				defer func() {
					if err := os.Remove(filename); err != nil {
						panic(err)
					}
				}()
				if os.WriteFile(filename, []byte(sampleKubeConfig), os.ModePerm) != nil {
					panic("Cannot create file for testing")
				}

				_, err = K8sClientHelper{}.GetClient(filename)
				Expect(err).ShouldNot(HaveOccurred())
			})
		})
	})

	Context("In-cluster", func() {
		It("should raise error", func() {
			_, err := K8sClientHelper{}.GetClient("")
			if FileExists(`/var/run/secrets/kubernetes.io/serviceaccount/token`) {
				Expect(err).ShouldNot(HaveOccurred())
			} else {
				Expect(err).Should(HaveOccurred())
			}
		})
	})
})
