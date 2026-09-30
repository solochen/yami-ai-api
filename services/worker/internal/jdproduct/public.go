package jdproduct

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"regexp"
	"sort"
	"strings"
	"time"
)

const mobileUserAgent = "Mozilla/5.0 (iPhone; CPU iPhone OS 17_5 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.5 Mobile/15E148 Safari/604.1"

var (
	skuNamePattern     = regexp.MustCompile(`"skuName"\s*:\s*"([^"]+)"`)
	detailImagePattern = regexp.MustCompile(`(?i)/sku/(?:s\d+x\d+_)?(jfs/[a-z0-9_./-]+\.(?:jpe?g|png|webp))`)
	packingPattern     = regexp.MustCompile(`(?s)包装清单</span>.{0,500}?class="content-block">\s*([^<]+)`)
)

type publicItem struct {
	Item struct {
		SKU         string              `json:"skuId"`
		Name        string              `json:"skuName"`
		Image       []string            `json:"image"`
		SaleProp    map[string]string   `json:"saleProp"`
		SalePropSeq map[string][]string `json:"salePropSeq"`
		Colors      []publicColor       `json:"newColorSize"`
	} `json:"item"`
}

type publicColor struct {
	Color     string `json:"color"`
	Size      string `json:"size"`
	ImagePath string `json:"imagePath"`
	SKU       string `json:"skuId"`
}

type specGroup struct {
	Atts []struct {
		Name string   `json:"attName"`
		Vals []string `json:"vals"`
	} `json:"atts"`
}

type specDocument struct {
	PropGroups []specGroup `json:"propGroups"`
}

func Load(ctx context.Context, sku string) (Product, error) {
	product, err := loadPublicProduct(ctx, sku)
	if err == nil && len(product.Images) > 0 {
		if product.SKU == "" {
			product.SKU = sku
		}
		return product, nil
	}
	ware, graphic, captureErr := Capture(ctx, "https://item.jd.com/"+sku+".html")
	if captureErr != nil {
		if err != nil {
			return Product{}, err
		}
		return Product{}, captureErr
	}
	parsed, parseErr := Parse(ware, graphic)
	if parseErr != nil || len(parsed.Images) == 0 {
		return Product{}, ErrPageUnavailable
	}
	if parsed.SKU == "" {
		parsed.SKU = sku
	}
	return parsed, nil
}

func loadPublicProduct(ctx context.Context, sku string) (Product, error) {
	client := publicPageClient()
	wareHTML, wareErr := getPublicPage(ctx, client, "https://item.m.jd.com/ware/view.action?wareId="+sku)
	graphicHTML, graphicErr := getPublicPage(ctx, client, "https://in.m.jd.com/product/graphext/"+sku+".html")
	if wareErr != nil && graphicErr != nil {
		return Product{}, ErrPageUnavailable
	}
	product := Product{SKU: sku}
	if wareErr == nil {
		applyWareView(&product, wareHTML)
	}
	if graphicErr == nil {
		applyGraphext(&product, graphicHTML)
	}
	product.Title = truncateRunes(strings.TrimSpace(product.Title), 120)
	product.Images = dedupeImages(product.Images)
	if len(product.Images) == 0 {
		return Product{}, ErrPageUnavailable
	}
	return product, nil
}

func applyWareView(product *Product, html string) {
	if name := longestSKUName(html); name != "" {
		product.Title = name
	}
	raw := balancedObject(html, "window._itemOnly = (")
	if raw == "" {
		return
	}
	var item publicItem
	if json.Unmarshal(looseJSON(raw), &item) != nil {
		return
	}
	if item.Item.SKU != "" {
		product.SKU = item.Item.SKU
	}
	labels := map[string]string{}
	for _, color := range item.Item.Colors {
		if image, ok := canonicalImage(color.ImagePath); ok {
			labels[image.key] = colorLabel(color, item.Item.SaleProp)
		}
	}
	currentLabel := ""
	for _, color := range item.Item.Colors {
		if color.SKU == product.SKU {
			currentLabel = colorLabel(color, item.Item.SaleProp)
			break
		}
	}
	for _, path := range item.Item.Image {
		image, ok := canonicalImage(path)
		if !ok {
			continue
		}
		spec := labels[image.key]
		if spec == "" {
			spec = currentLabel
		}
		product.Images = append(product.Images, Image{Role: "gallery", Spec: spec, SourceURL: image.url})
	}
	for _, color := range item.Item.Colors {
		image, ok := canonicalImage(color.ImagePath)
		if !ok {
			continue
		}
		product.Images = append(product.Images, Image{Role: "gallery", Spec: colorLabel(color, item.Item.SaleProp), SourceURL: image.url})
	}
	product.SaleSpecs = saleSpecsFromItem(item)
}

func applyGraphext(product *Product, html string) {
	if raw := hiddenValue(html, "wareGuigNew"); raw != "" {
		var doc specDocument
		if json.Unmarshal(looseJSON(raw), &doc) == nil {
			for _, group := range doc.PropGroups {
				for _, att := range group.Atts {
					value := cleanText(strings.Join(att.Vals, "、"))
					name := cleanText(att.Name)
					if name == "" || value == "" {
						continue
					}
					product.Attributes = appendUniqueAttr(product.Attributes, Attribute{Name: name, Value: truncateRunes(value, 200)})
				}
			}
		}
	}
	if match := packingPattern.FindStringSubmatch(html); len(match) == 2 {
		if value := cleanText(match[1]); value != "" {
			product.Attributes = appendUniqueAttr(product.Attributes, Attribute{Name: "包装清单", Value: truncateRunes(value, 200)})
		}
	}
	seen := map[string]bool{}
	for _, match := range detailImagePattern.FindAllStringSubmatch(html, -1) {
		image, ok := canonicalImage(match[1])
		if !ok || seen[image.key] {
			continue
		}
		seen[image.key] = true
		product.Images = append(product.Images, Image{Role: "detail", SourceURL: image.url})
	}
}

func saleSpecsFromItem(item publicItem) []SaleSpec {
	ids := make([]string, 0, len(item.Item.SalePropSeq))
	for id := range item.Item.SalePropSeq {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	var specs []SaleSpec
	for _, id := range ids {
		values := item.Item.SalePropSeq[id]
		if len(values) == 0 {
			continue
		}
		name := cleanText(item.Item.SaleProp[id])
		if name == "" {
			name = "规格"
		}
		specs = append(specs, SaleSpec{Name: name, Values: values})
	}
	return specs
}

func colorLabel(color publicColor, saleProp map[string]string) string {
	var parts []string
	if text := cleanText(color.Color); text != "" {
		parts = append(parts, firstNonEmpty(cleanText(saleProp["1"]), "规格")+":"+text)
	}
	if text := cleanText(color.Size); text != "" {
		parts = append(parts, firstNonEmpty(cleanText(saleProp["2"]), "尺码")+":"+text)
	}
	return strings.Join(parts, " / ")
}

func longestSKUName(html string) string {
	best := ""
	for _, match := range skuNamePattern.FindAllStringSubmatch(html, -1) {
		name := cleanText(match[1])
		if len([]rune(name)) > len([]rune(best)) {
			best = name
		}
	}
	return best
}

func hiddenValue(html, name string) string {
	token := `id="` + name + `"`
	index := strings.Index(html, token)
	if index < 0 {
		return ""
	}
	rest := html[index:]
	marker := `value='`
	start := strings.Index(rest, marker)
	if start < 0 {
		return ""
	}
	rest = rest[start+len(marker):]
	end := strings.Index(rest, `'`)
	if end < 0 {
		return ""
	}
	return rest[:end]
}

func balancedObject(html, marker string) string {
	index := strings.Index(html, marker)
	if index < 0 {
		return ""
	}
	start := strings.Index(html[index:], "{")
	if start < 0 {
		return ""
	}
	start += index
	depth := 0
	for i, char := range html[start:] {
		switch char {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return html[start : start+i+1]
			}
		}
	}
	return ""
}

func looseJSON(raw string) []byte {
	raw = strings.TrimSpace(raw)
	for {
		next := trailingComma.ReplaceAllString(raw, "$1")
		if next == raw {
			return []byte(next)
		}
		raw = next
	}
}

var trailingComma = regexp.MustCompile(`,\s*([}\]])`)

func getPublicPage(ctx context.Context, client *http.Client, rawURL string) (string, error) {
	reqCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, rawURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", mobileUserAgent)
	req.Header.Set("Accept", "text/html")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9")
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.Request != nil && resp.Request.URL != nil && strings.Contains(resp.Request.URL.Path, "risk_handler") {
		return "", ErrPageUnavailable
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("商品页 HTTP %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return "", err
	}
	text := string(body)
	if strings.Contains(text, "京东验证") || strings.Contains(text, "risk_handler") {
		return "", ErrPageUnavailable
	}
	return text, nil
}

func publicPageClient() *http.Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.DialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
		return dialChecked(ctx, network, address, allowedPublicHost, "不是京东商品页")
	}
	return &http.Client{
		Timeout:   25 * time.Second,
		Transport: transport,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 4 {
				return fmt.Errorf("商品页重定向次数过多")
			}
			if strings.Contains(req.URL.Path, "risk_handler") || !allowedPublicHost(req.URL.Hostname()) {
				return ErrPageUnavailable
			}
			return nil
		},
	}
}

func allowedPublicHost(host string) bool {
	host = strings.ToLower(strings.TrimSuffix(host, "."))
	switch host {
	case "item.m.jd.com", "in.m.jd.com", "item.jd.com":
		return true
	default:
		return false
	}
}
