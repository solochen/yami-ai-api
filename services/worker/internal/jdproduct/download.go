package jdproduct

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

const (
	maxImageBytes = 12 << 20
	maxTotalBytes = 80 << 20
)

type DownloadedImage struct {
	Image
	Body        []byte
	ContentType string
}

func Download(ctx context.Context, images []Image) ([]DownloadedImage, error) {
	client := safeImageClient()
	var saved []DownloadedImage
	var total int
	for _, image := range images {
		if err := ctx.Err(); err != nil {
			return saved, err
		}
		body, contentType, err := fetchImage(ctx, client, image.SourceURL)
		if err != nil || len(body) == 0 {
			continue
		}
		if total+len(body) > maxTotalBytes {
			break
		}
		total += len(body)
		imageCopy := image
		saved = append(saved, DownloadedImage{Image: imageCopy, Body: body, ContentType: contentType})
	}
	if len(saved) == 0 {
		return nil, ErrPageUnavailable
	}
	return saved, nil
}

func fetchImage(ctx context.Context, client *http.Client, rawURL string) ([]byte, string, error) {
	image, ok := canonicalImage(rawURL)
	if !ok {
		return nil, "", fmt.Errorf("不是商品图片")
	}
	reqCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, image.url, nil)
	if err != nil {
		return nil, "", err
	}
	req.Header.Set("User-Agent", browserUserAgent)
	req.Header.Set("Referer", "https://item.jd.com/")
	resp, err := client.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, "", fmt.Errorf("图片 HTTP %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxImageBytes+1))
	if err != nil {
		return nil, "", err
	}
	if len(body) == 0 || len(body) > maxImageBytes {
		return nil, "", fmt.Errorf("图片大小无效")
	}
	contentType := strings.ToLower(strings.TrimSpace(strings.Split(resp.Header.Get("Content-Type"), ";")[0]))
	detected := http.DetectContentType(body)
	if !strings.HasPrefix(contentType, "image/") {
		contentType = detected
	}
	if !strings.HasPrefix(contentType, "image/") {
		return nil, "", fmt.Errorf("返回的不是图片")
	}
	return body, contentType, nil
}

func safeImageClient() *http.Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.DialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
		return dialChecked(ctx, network, address, allowedImageHost, "图片地址不在京东图床")
	}
	return &http.Client{
		Timeout:   25 * time.Second,
		Transport: transport,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 3 {
				return fmt.Errorf("图片重定向次数过多")
			}
			if !allowedImageHost(req.URL.Hostname()) {
				return fmt.Errorf("图片重定向离开京东图床")
			}
			return nil
		},
	}
}

func allowedImageHost(host string) bool {
	host = strings.ToLower(strings.TrimSuffix(host, "."))
	return host == "360buyimg.com" || strings.HasSuffix(host, ".360buyimg.com")
}

func dialChecked(ctx context.Context, network, address string, isDestination func(string) bool, destinationError string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return nil, err
	}
	addresses, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	if err != nil || len(addresses) == 0 {
		return nil, fmt.Errorf("无法解析地址")
	}
	destination := isDestination(host)
	if !destination && destinationError != "" && !isProxyDial(host) {
		return nil, fmt.Errorf("%s", destinationError)
	}
	for _, resolved := range addresses {
		ip := resolved.IP
		if destination && blockedIP(ip) {
			return nil, fmt.Errorf("禁止访问本机或内网地址")
		}
		if !destination && blockedIP(ip) && !ip.IsLoopback() {
			return nil, fmt.Errorf("禁止访问内网地址")
		}
	}
	dialer := &net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second}
	return dialer.DialContext(ctx, network, net.JoinHostPort(addresses[0].IP.String(), port))
}

func isProxyDial(host string) bool {
	proxy := strings.TrimSpace(os.Getenv("HTTPS_PROXY"))
	if proxy == "" {
		proxy = strings.TrimSpace(os.Getenv("HTTP_PROXY"))
	}
	if proxy == "" {
		proxy = strings.TrimSpace(os.Getenv("https_proxy"))
	}
	if proxy == "" {
		proxy = strings.TrimSpace(os.Getenv("http_proxy"))
	}
	if proxy == "" {
		return false
	}
	parsed, err := url.Parse(proxy)
	if err != nil {
		return false
	}
	return strings.EqualFold(parsed.Hostname(), host)
}

func blockedIP(ip net.IP) bool {
	if ip == nil || ip.IsUnspecified() || ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsMulticast() {
		return true
	}
	if v4 := ip.To4(); v4 != nil && v4[0] == 100 && v4[1]&0xc0 == 64 {
		return true
	}
	return false
}
