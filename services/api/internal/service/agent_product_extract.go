package service

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

var productIDPattern = regexp.MustCompile(`^[1-9][0-9]{4,19}$`)

func normalizeProductImageExtractInputs(inputs map[string]interface{}) error {
	if inputs == nil {
		return fmt.Errorf("请输入京东商品链接")
	}
	raw := strings.TrimSpace(stringValue(inputs["product_url"]))
	if raw == "" {
		raw = strings.TrimSpace(stringValue(inputs["url"]))
	}
	link, err := parseProductURL(raw)
	if err != nil {
		return err
	}
	inputs["product_url"] = link.URL
	inputs["platform"] = link.Platform
	inputs["product_id"] = link.ID
	inputs["sku_id"] = link.ID
	return nil
}

type productLink struct {
	Platform string
	ID       string
	URL      string
}

func parseProductURL(raw string) (productLink, error) {
	raw = strings.TrimSpace(raw)
	if start := strings.Index(raw, "http"); start > 0 {
		raw = raw[start:]
	}
	if end := strings.IndexAny(raw, " \t\r\n"); end >= 0 {
		raw = raw[:end]
	}
	raw = strings.Trim(raw, `"'<>，。；;）)]】`)
	parsed, err := url.Parse(raw)
	if err != nil || parsed.User != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Hostname() == "" {
		return productLink{}, fmt.Errorf("请输入京东商品链接")
	}
	host := strings.ToLower(strings.TrimSuffix(parsed.Hostname(), "."))
	if !jdProductHost(host) {
		return productLink{}, fmt.Errorf("仅支持京东商品链接")
	}
	id := productIDFromURL(parsed)
	if !productIDPattern.MatchString(id) {
		return productLink{}, fmt.Errorf("商品编号无效")
	}
	return productLink{Platform: "jd", ID: id, URL: "https://item.jd.com/" + id + ".html"}, nil
}

func jdProductHost(host string) bool {
	switch host {
	case "item.jd.com", "item.m.jd.com", "item.jd.hk", "npcitem.jd.hk", "mitem.jd.hk":
		return true
	default:
		return false
	}
}

func productIDFromURL(parsed *url.URL) string {
	for _, key := range []string{"id", "item_id", "itemId", "sku", "skuId", "wareId", "product_id", "promotion_id"} {
		if value := strings.TrimSpace(parsed.Query().Get(key)); productIDPattern.MatchString(value) {
			return value
		}
	}
	path := strings.Trim(parsed.EscapedPath(), "/")
	path = strings.TrimSuffix(path, ".html")
	path = strings.TrimSuffix(path, ".htm")
	if slash := strings.LastIndex(path, "/"); slash >= 0 {
		path = path[slash+1:]
	}
	if strings.HasPrefix(path, "product/") {
		path = strings.TrimPrefix(path, "product/")
	}
	if productIDPattern.MatchString(path) {
		return path
	}
	return ""
}
