package storage

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

type LocalClient struct {
	root      string
	publicURL string
}

func NewLocal(root, publicURL string) (*LocalClient, error) {
	if strings.TrimSpace(root) == "" {
		root = os.Getenv("LOCAL_STORAGE_DIR")
	}
	if strings.TrimSpace(root) == "" {
		root = "../../data/uploads"
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(abs, 0o755); err != nil {
		return nil, err
	}
	return &LocalClient{root: abs, publicURL: strings.TrimRight(publicURL, "/")}, nil
}

func (c *LocalClient) Upload(ctx context.Context, objectName, contentType string, r io.Reader, size int64) (string, error) {
	target, err := c.safePath(objectName)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return "", err
	}
	f, err := os.Create(target)
	if err != nil {
		return "", err
	}
	defer f.Close()
	if _, err := io.Copy(f, r); err != nil {
		return "", err
	}
	return c.publicURLFor(objectName), nil
}

func (c *LocalClient) ReadAll(ctx context.Context, objectName string, maxBytes int64) ([]byte, error) {
	target, err := c.safePath(objectName)
	if err != nil {
		return nil, err
	}
	file, err := os.Open(target)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, maxBytes+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > maxBytes {
		return nil, fmt.Errorf("object exceeds size limit")
	}
	return data, nil
}

func (c *LocalClient) ObjectKeyFromURL(rawURL string) string {
	raw := strings.TrimSpace(rawURL)
	if raw == "" {
		return ""
	}
	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	base, _ := url.Parse(c.publicURL)
	basePath := ""
	if base != nil {
		basePath = strings.Trim(strings.TrimPrefix(base.Path, "/"), "/")
	}
	path := strings.Trim(strings.TrimPrefix(u.Path, "/"), "/")
	if u.Host != "" {
		if base != nil && base.Host != "" {
			if !strings.EqualFold(u.Host, base.Host) {
				return ""
			}
		} else if basePath == "" || !strings.HasPrefix(path, basePath+"/") {
			// publicURL 无 host 时，只接受带代理前缀的绝对地址。
			return ""
		}
	}
	// 相对媒体地址（/media/<key>）也要能反查对象 key。
	if basePath != "" {
		path = strings.TrimPrefix(path, basePath+"/")
	}
	path = strings.Trim(path, "/")
	if _, err := c.safePath(path); err != nil {
		return ""
	}
	if path == "" {
		return ""
	}
	return path
}

func (c *LocalClient) publicURLFor(objectName string) string {
	key := strings.Trim(strings.ReplaceAll(objectName, "\\", "/"), "/")
	parts := strings.Split(key, "/")
	for i := range parts {
		parts[i] = url.PathEscape(parts[i])
	}
	return fmt.Sprintf("%s/%s", c.publicURL, strings.Join(parts, "/"))
}

func (c *LocalClient) safePath(objectName string) (string, error) {
	key := strings.Trim(strings.ReplaceAll(objectName, "\\", "/"), "/")
	if key == "" || strings.Contains(key, "..") {
		return "", fmt.Errorf("invalid object name")
	}
	target := filepath.Join(c.root, filepath.FromSlash(key))
	rel, err := filepath.Rel(c.root, target)
	if err != nil || strings.HasPrefix(rel, "..") || filepath.IsAbs(rel) {
		return "", fmt.Errorf("invalid object name")
	}
	return target, nil
}
