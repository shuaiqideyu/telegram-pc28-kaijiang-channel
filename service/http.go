package service

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const chromeUA = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/136.0.0.0 Safari/537.36"

func ipv4Transport() *http.Transport {
	dialer := &net.Dialer{Timeout: 5 * time.Second, KeepAlive: 30 * time.Second}
	return &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			return dialer.DialContext(ctx, "tcp4", addr)
		},
		MaxIdleConnsPerHost: 2,
		IdleConnTimeout:     120 * time.Second,
		ForceAttemptHTTP2:   true,
		DisableCompression:  true,
		WriteBufferSize:     4096,
		ReadBufferSize:      4096,
	}
}

func redactYu28Err(path string, err error) error {
	if err == nil {
		return nil
	}
	var ue *url.Error
	if errors.As(err, &ue) && ue.Err != nil {
		err = ue.Err
	}
	msg := err.Error()
	if strings.Contains(msg, "key=") || strings.Contains(msg, "api_key=") || (drawAPIKey != "" && strings.Contains(msg, drawAPIKey)) {
		return fmt.Errorf("yu28 %s", path)
	}
	return fmt.Errorf("yu28 %s: %v", path, err)
}

func httpGet(client *http.Client, rawURL string) ([]byte, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("yu28 request")
	}
	if drawAPIKey != "" {
		q := u.Query()
		q.Del("api_key")
		q.Set("key", drawAPIKey)
		u.RawQuery = q.Encode()
	}
	req, err := http.NewRequest(http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("yu28 %s", u.Path)
	}
	req.Header.Set("User-Agent", chromeUA)
	req.Header.Set("Accept", "application/json, text/plain, */*")

	resp, err := client.Do(req)
	if err != nil {
		return nil, redactYu28Err(u.Path, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		io.Copy(io.Discard, resp.Body)
		return nil, fmt.Errorf("status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("yu28 %s", u.Path)
	}
	return body, nil
}
