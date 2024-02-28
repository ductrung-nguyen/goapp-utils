package utils

import (
	"compress/gzip"
	"crypto/tls"
	"io"
	"net/http"
	"net/http/cookiejar"
	urlUtils "net/url"
	"time"
)

type HttpBasicAuth struct {
	Username string
	Password string
}

type RequestOptions struct {
	// whether to skip insecure certificate verification.
	SkipInsecureVerify bool

	// Auth represents the HTTP basic authentication credentials.
	Auth HttpBasicAuth

	// Timeout represents the request timeout.
	Timeout time.Duration
}

// NewRequestOptions creates a new RequestOptions struct with the specified parameters.
// It takes a boolean value skipInsecurityVerify to indicate whether to skip insecure certificate verification,
// an HttpBasicAuth struct auth for HTTP basic authentication, and a time.Duration timeout for the request timeout.
// It returns a RequestOptions struct.
func NewRequestOptions(skipInsecurityVerify bool, auth HttpBasicAuth, timeout time.Duration) RequestOptions {
	return RequestOptions{
		SkipInsecureVerify: skipInsecurityVerify,
		Auth:               auth,
		Timeout:            timeout,
	}
}

// HttpClientInterface is a simple interface that defines the functions of a HTTP client
type HttpClientInterface interface {
	SendRequest(
		url string,
		cookieJar *cookiejar.Jar,
		header map[string]string,
		method string,
		payload io.Reader,
		queryParams map[string]string,
		options RequestOptions,
	) (content []byte, statusCode int, err error)
}

// RealHTTPClient implements the real http client service
type RealHTTPClient struct {
}

var _ HttpClientInterface = RealHTTPClient{}

// SendRequest sends a get request and return the response
// if the response is compressed, un-compress it first and then return
func (RealHTTPClient) SendRequest(
	url string,
	cookieJar *cookiejar.Jar,
	header map[string]string,
	method string,
	payload io.Reader,
	queryParams map[string]string,
	options RequestOptions,
) (content []byte, statusCode int, err error) {
	_, err = urlUtils.Parse(url)
	if err != nil {
		return nil, 0, err
	}

	client := &http.Client{
		Transport: &http.Transport{
			MaxConnsPerHost: 30,
			TLSClientConfig: &tls.Config{InsecureSkipVerify: options.SkipInsecureVerify},
		},
		Timeout: options.Timeout,
	}
	if cookieJar != nil {
		client.Jar = cookieJar
	}
	if method == "" {
		method = http.MethodGet
	}

	req, err := http.NewRequest(method, url, payload)
	if err != nil {
		return nil, 0, err
	}
	if options.Auth.Username != "" || options.Auth.Password != "" {
		req.SetBasicAuth(options.Auth.Username, options.Auth.Password)
	}

	if len(queryParams) > 0 {
		q := req.URL.Query()
		for k, v := range queryParams {
			q.Add(k, v)
		}
		req.URL.RawQuery = q.Encode()
	}

	for k, v := range header {
		req.Header.Set(k, v)
	}

	res, err := client.Do(req)
	if err != nil {
		return nil, 0, err
	}

	defer res.Body.Close()

	var reader io.ReadCloser
	switch res.Header.Get("Content-Encoding") {
	case "gzip":
		reader, err = gzip.NewReader(res.Body)
		defer reader.Close()
	default:
		reader = res.Body
	}

	contentBytes, err := io.ReadAll(reader)
	res.Body.Close()

	if err != nil {
		return nil, res.StatusCode, err
	}

	return contentBytes, res.StatusCode, nil
}
