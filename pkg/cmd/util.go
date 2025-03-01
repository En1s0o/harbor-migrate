package cmd

import (
	"context"
	"crypto/tls"
	"io"
	"net"
	"net/http"
	"time"
)

var httpClient *http.Client

func init() {
	transport := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		ForceAttemptHTTP2:     true,
		TLSClientConfig:       &tls.Config{InsecureSkipVerify: true},
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}
	httpClient = &http.Client{
		Transport: transport,
	}
}

func makeHTTPRequest(ctx context.Context, method, url, username, password string, body io.Reader) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err == nil {
		req.Header.Set("User-Agent", "harbor-migrate/1.0.0")
		req.Header.Set("Content-Type", "application/json; charset=utf-8")
		req.Header.Set("Connection", "Close")
		req.SetBasicAuth(username, password)
	}
	return req, err
}
