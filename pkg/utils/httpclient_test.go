package utils

import (
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/ghttp"
)

var _ = Describe("Client", func() {
	var (
		server     *ghttp.Server
		statusCode int
		body       string
		path       string
		addr       string
		username   string
		password   string
	)
	BeforeEach(func() {
		// start a test http server
		server = ghttp.NewServer()
	})
	AfterEach(func() {
		server.Close()
	})
	Context("When given empty url", func() {
		BeforeEach(func() {
			addr = ""
		})
		It("Returns the empty path", func() {
			httpClient := &RealHTTPClient{}
			_, _, err := httpClient.SendRequest(addr, nil, nil, "GET", nil, nil, true, "", "", 1*time.Second)
			Expect(err).Should(HaveOccurred())
		})
	})
	Context("When given unsupported protocol scheme", func() {
		BeforeEach(func() {
			addr = "tcp://localhost"
		})
		It("Returns the empty path", func() {
			httpClient := &RealHTTPClient{}
			_, _, err := httpClient.SendRequest(addr, nil, nil, "GET", nil, nil, true, "", "", 1*time.Second)
			Expect(err).Should(HaveOccurred())
		})
	})

	Context("When accesing wrong URL", func() {
		BeforeEach(func() {
			path = "/"
			addr = "http://" + server.Addr() + "WRONG" + path
		})
		It("Returns the empty path", func() {
			httpClient := &RealHTTPClient{}
			_, _, err := httpClient.SendRequest(addr, nil, nil, "GET", nil, nil, true, "", "", 1*time.Second)
			Expect(err).Should(HaveOccurred())
		})
	})
	Context("When get request is sent to empty path", func() {
		BeforeEach(func() {
			statusCode = 200
			path = "/"
			body = "Hi there, the end point is :!"
			addr = "http://" + server.Addr() + path
			server.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", path),
					ghttp.RespondWithPtr(&statusCode, &body),
				))
		})
		It("Returns the empty path", func() {
			httpClient := &RealHTTPClient{}
			gotBody, _, err := httpClient.SendRequest(addr, nil, nil, "GET", nil, nil, true, "", "", 1*time.Second)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(gotBody).To(Equal(body))
		})
	})

	Context("When get request is sent to empty path with default request", func() {
		BeforeEach(func() {
			statusCode = 200
			path = "/"
			body = "Hi there, the end point is :!"
			addr = "http://" + server.Addr() + path
			server.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", path),
					ghttp.RespondWithPtr(&statusCode, &body),
				))
		})
		It("Returns the empty path", func() {
			httpClient := &RealHTTPClient{}
			gotBody, _, err := httpClient.SendRequest(addr, nil, nil, "", nil, nil, true, "", "", 1*time.Second)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(gotBody).To(Equal(body))
		})
	})
	Context("When get request is sent to hello path", func() {
		BeforeEach(func() {
			statusCode = 200
			path = "/hello"
			body = "Hi there, the end point is :hello!"
			addr = "http://" + server.Addr() + path
			server.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", path),
					ghttp.RespondWithPtr(&statusCode, &body),
				))
		})
		It("Returns the hello path", func() {
			httpClient := &RealHTTPClient{}
			gotBody, _, err := httpClient.SendRequest(addr, nil, nil, "GET", nil, nil, true, "", "", 1*time.Second)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(gotBody).To(Equal(body))
		})
	})

	Context("When get request is sent to hello path with cookie supported", func() {
		BeforeEach(func() {
			statusCode = 200
			path = "/hello"
			body = "Hi there, the end point is :hello!"
			addr = "http://" + server.Addr() + path
			server.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", path),
					ghttp.RespondWithPtr(&statusCode, &body),
				))
		})
		It("Returns the hello path", func() {
			httpClient := &RealHTTPClient{}
			cookie, _ := cookiejar.New(nil)
			gotBody, _, err := httpClient.SendRequest(addr, cookie, nil, "GET", nil, nil, true, "", "", 1*time.Second)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(gotBody).To(Equal(body))
		})
	})

	Context("When get request is sent to hello path with params", func() {
		BeforeEach(func() {
			statusCode = 200
			path = "/hello"
			body = "Hi there, the end point is :hello!"
			addr = "http://" + server.Addr() + path
			server.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", path),
					ghttp.VerifyForm(url.Values{
						"q":    []string{"logging"},
						"sort": []string{"asc"},
					}),
					ghttp.RespondWithPtr(&statusCode, &body),
				))
		})
		It("Returns the hello path", func() {
			httpClient := &RealHTTPClient{}
			gotBody, _, err := httpClient.SendRequest(addr, nil, nil, "GET", nil, map[string]string{
				"q":    "logging",
				"sort": "asc",
			}, true, "", "", 1*time.Second)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(gotBody).To(Equal(body))
		})
	})

	Context("When get request is sent to hello path with headers", func() {
		BeforeEach(func() {
			statusCode = 200
			path = "/hello"
			body = "Hi there, the end point is :hello!"
			addr = "http://" + server.Addr() + path
			server.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", path),
					ghttp.VerifyHeader(http.Header{
						"header1": []string{"value1"},
						"header2": []string{"value2"},
					}),
					ghttp.RespondWithPtr(&statusCode, &body),
				))
		})
		It("Returns the hello path", func() {
			httpClient := &RealHTTPClient{}
			gotBody, _, err := httpClient.SendRequest(addr, nil, map[string]string{
				"header1": "value1",
				"header2": "value2",
			}, "GET", nil, nil, true, "", "", 1*time.Second)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(gotBody).To(Equal(body))
		})
	})

	Context("When get request is sent to hello path using basic authentication", func() {
		BeforeEach(func() {
			statusCode = 200
			path = "/hello"
			body = "Hi there, the end point is :hello!"
			addr = "http://" + server.Addr() + path
			username = "test_username"
			password = "test_password"
			server.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", path),
					ghttp.VerifyBasicAuth(username, password),
					ghttp.RespondWithPtr(&statusCode, &body),
				))
		})
		It("Returns the hello path with correct credentials", func() {
			httpClient := &RealHTTPClient{}
			gotBody, _, err := httpClient.SendRequest(addr, nil, nil, "GET", nil, nil, true, username, password, 1*time.Second)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(gotBody).To(Equal(body))
		})
	})
	Context("When get request is sent to read path but there is no file", func() {
		BeforeEach(func() {
			statusCode = 500
			path = "/read"
			body = "open data.txt: no such file or directory\r\n"
			addr = "http://" + server.Addr() + path
			server.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", path),
					ghttp.RespondWithPtr(&statusCode, &body),
				))
		})
		It("Returns internal server error", func() {
			httpClient := &RealHTTPClient{}
			_, _, err := httpClient.SendRequest(addr, nil, nil, "GET", nil, nil, true, "", "", 1*time.Second)
			Expect(err).ShouldNot(HaveOccurred())
		})
	})
	// Context("When get request is sent to read path but file exists", func() {
	// 	BeforeEach(func() {
	// 		file, err := os.Create("data.txt")
	// 		Expect(err).NotTo(HaveOccurred())
	// 		body = []byte("Hi there!")
	// 		file.Write(body)
	// 		statusCode = 200
	// 		path = "/read"
	// 		addr = "http://" + server.Addr() + path
	// 		server.AppendHandlers(
	// 			ghttp.CombineHandlers(
	// 				ghttp.VerifyRequest("GET", path),
	// 				ghttp.RespondWithPtr(&statusCode, &body),
	// 			))
	// 	})
	// 	AfterEach(func() {
	// 		err := os.Remove("data.txt")
	// 		Expect(err).NotTo(HaveOccurred())
	// 	})
	// 	It("Reads data from file successfully", func() {
	// 		bdy, err := getResponse(addr)
	// 		Expect(err).ShouldNot(HaveOccurred())
	// 		Expect(bdy).To(Equal(body))
	// 	})
	// })
})
