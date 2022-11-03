package utils

import (
	"compress/gzip"
	"crypto/tls"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/cookiejar"
	urlUtils "net/url"
	"time"
)

// HttpClientInterface is a simple interface that defines the functions of a HTTP client
type HttpClientInterface interface {
	SendRequest(
		url string,
		cookieJar *cookiejar.Jar,
		header map[string]string,
		method string,
		payload io.Reader,
		queryParams map[string]string,
		skipInsecureVerify bool,
		username string,
		password string,
		timeout time.Duration,
	) (content string, statusCode int, err error)
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
	skipInsecureVerify bool,
	username string,
	password string,
	timeout time.Duration,
) (content string, statusCode int, err error) {
	_, err = urlUtils.Parse(url)
	if err != nil {
		return "", 0, err
	}

	client := &http.Client{
		Transport: &http.Transport{
			MaxConnsPerHost: 30,
			TLSClientConfig: &tls.Config{InsecureSkipVerify: skipInsecureVerify},
		},
		Timeout: timeout,
	}
	if cookieJar != nil {
		client.Jar = cookieJar
	}
	if method == "" {
		method = http.MethodGet
	}

	req, err := http.NewRequest(method, url, payload)
	if err != nil {
		return "", 0, err
	}
	if username != "" || password != "" {
		req.SetBasicAuth(username, password)
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
		return "", 0, err
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

	contentBytes, err := ioutil.ReadAll(reader)
	res.Body.Close()

	if err != nil {
		return "", res.StatusCode, err
	}

	return string(contentBytes), res.StatusCode, nil
}
