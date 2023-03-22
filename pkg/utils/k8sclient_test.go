package utils

import (
	"fmt"
	"io/ioutil"
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
    server: https://api.nld7.paas.westeurope.tstcur.az.amadeus.net:6443
  name: api-nld7-paas-westeurope-tstcur-az-amadeus-net:6443
contexts:
- context:
    cluster: api-nld7-paas-westeurope-tstcur-az-amadeus-net:6443
    namespace: splunk
    user: pdnguyen
  name: splunk/api-nld7-paas-westeurope-tstcur-az-amadeus-net:6443/pdnguyen
current-context: splunk/api-nld7-paas-westeurope-tstcur-az-amadeus-net:6443/pdnguyen
kind: Config
preferences: {}
users:
- name: pdnguyen
  user:
    token: sha256~sample-token
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
				defer f.Close()
				filename := f.Name()
				defer os.Remove(filename)
				if ioutil.WriteFile(filename, []byte(sampleKubeConfig), os.ModePerm) != nil {
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
