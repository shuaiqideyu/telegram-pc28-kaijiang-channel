package service

import (
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"time"
)

const chromeUA = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/136.0.0.0 Safari/537.36"

func resolveDNS(host string) string {
	ips, err := net.LookupHost(host)
	if err != nil || len(ips) == 0 {
		log.Printf("[DNS] %s 解析失败: %v", host, err)
		return ""
	}
	log.Printf("[DNS] %s -> %s", host, ips[0])
	return ips[0]
}

func pinnedTransport(cachedIP string) *http.Transport {
	dialer := &net.Dialer{Timeout: 5 * time.Second, KeepAlive: 30 * time.Second}
	dialFn := dialer.DialContext
	if cachedIP != "" {
		dialFn = func(ctx context.Context, network, addr string) (net.Conn, error) {
			_, port, _ := net.SplitHostPort(addr)
			return dialer.DialContext(ctx, network, net.JoinHostPort(cachedIP, port))
		}
	}
	return &http.Transport{
		DialContext:         dialFn,
		MaxIdleConnsPerHost: 2,
		IdleConnTimeout:     120 * time.Second,
		ForceAttemptHTTP2:   true,
		DisableCompression:  true,
		WriteBufferSize:     4096,
		ReadBufferSize:      4096,
	}
}

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

func httpGet(client *http.Client, url string) ([]byte, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", chromeUA)
	req.Header.Set("Accept", "application/json, text/plain, */*")
	if drawAPIKey != "" {
		req.Header.Set("X-Api-Key", drawAPIKey)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		io.Copy(io.Discard, resp.Body)
		return nil, fmt.Errorf("status %d", resp.StatusCode)
	}
	return io.ReadAll(resp.Body)
}
