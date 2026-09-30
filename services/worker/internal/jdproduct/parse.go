package jdproduct

import (
	"encoding/json"
	"html"
	"regexp"
	"strconv"
	"strings"
)

const (
	maxGalleryImages = 40
	maxDetailImages  = 40
)

type SaleSpec struct {
	Name   string   `json:"name"`
	Values []string `json:"values"`
}

type Attribute struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type Image struct {
	Role      string `json:"role"`
	Spec      string `json:"spec,omitempty"`
	Index     int    `json:"index"`
	SourceURL string `json:"source_url"`
}

type Product struct {
	SKU        string      `json:"sku_id"`
	Title      string      `json:"title"`
	SaleSpecs  []SaleSpec  `json:"sale_specs"`
	Attributes []Attribute `json:"attributes"`
	Images     []Image     `json:"images"`
}

var (
	jfsPattern = regexp.MustCompile(`(?i)(?:s\d+x\d+_)?(jfs/[a-z0-9_./-]+\.(?:jpe?g|png|webp|gif|avif))`)
	htmlImage  = regexp.MustCompile(`(?i)(?:src|data-lazyload|data-src|data-original)\s*=\s*["']([^"']+)["']`)
	cssImage   = regexp.MustCompile(`(?i)url\(\s*(?:&quot;|["']?)([^"')]+)`)
)

func Parse(wareBody, graphicBody []byte) (Product, error) {
	var product Product
	ware := unwrapData(wareBody)
	graphic := unwrapData(graphicBody)
	if ware != nil {
		if page, ok := ware["pageConfigVO"].(map[string]any); ok {
			product.SKU = cleanText(firstNonEmpty(stringValue(page["skuid"]), stringValue(page["skuId"])))
			product.Title = cleanText(stringValue(page["skuName"]))
		}
		if product.SKU == "" {
			product.SKU = firstString(ware, "skuid", "skuId")
		}
		if product.Title == "" {
			product.Title = firstString(ware, "skuName", "wareName", "productName")
		}
		product.SaleSpecs = saleSpecs(ware)
		product.Attributes = attributes(ware)
		product.Images = append(product.Images, galleryImages(ware)...)
	}
	if graphic != nil {
		if qd := cleanText(stringValue(graphic["wareQD"])); qd != "" && !strings.Contains(qd, "<") {
			product.Attributes = appendUniqueAttr(product.Attributes, Attribute{Name: "包装清单", Value: qd})
		}
		product.Images = append(product.Images, detailImages(graphic)...)
	}
	product.Title = truncateRunes(strings.TrimSpace(product.Title), 120)
	product.Images = dedupeImages(product.Images)
	return product, nil
}

func unwrapData(body []byte) map[string]any {
	body = bytesTrim(body)
	if len(body) == 0 {
		return nil
	}
	var root any
	if err := json.Unmarshal(body, &root); err != nil {
		return nil
	}
	obj, ok := root.(map[string]any)
	if !ok {
		return nil
	}
	if data, ok := obj["data"].(map[string]any); ok {
		return data
	}
	return obj
}

func bytesTrim(body []byte) []byte {
	return []byte(strings.TrimSpace(string(body)))
}

func galleryImages(ware map[string]any) []Image {
	var images []Image
	specByKey := specImages(ware)
	if main, ok := ware["mainImageVO"].(map[string]any); ok {
		if area, ok := main["mainImageArea"].(map[string]any); ok {
			if image, ok := canonicalImage(stringValue(area["imageUrl"])); ok {
				images = append(images, Image{Role: "gallery", Spec: specByKey[image.key], SourceURL: image.url})
			}
		}
		if area, ok := main["carouselArea"].([]any); ok {
			for _, raw := range area {
				item, ok := raw.(map[string]any)
				if !ok {
					continue
				}
				image, ok := canonicalImage(stringValue(item["imageUrl"]))
				if !ok {
					continue
				}
				images = append(images, Image{Role: "gallery", Spec: specByKey[image.key], SourceURL: image.url})
			}
		}
	}
	for key, spec := range specByKey {
		found := false
		for _, image := range images {
			if imageKey(image.SourceURL) == key {
				found = true
				break
			}
		}
		if found {
			continue
		}
		if image, ok := canonicalImage(key); ok {
			images = append(images, Image{Role: "gallery", Spec: spec, SourceURL: image.url})
		}
	}
	return images
}

func specImages(ware map[string]any) map[string]string {
	out := map[string]string{}
	color, _ := ware["colorSizeVO"].(map[string]any)
	list, _ := color["colorSizeList"].([]any)
	for _, raw := range list {
		group, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		title := cleanText(stringValue(group["title"]))
		buttons, _ := group["buttons"].([]any)
		for _, buttonRaw := range buttons {
			button, ok := buttonRaw.(map[string]any)
			if !ok {
				continue
			}
			text := cleanText(firstNonEmpty(stringValue(button["text"]), stringValue(button["name"])))
			image, ok := canonicalImage(stringValue(button["imageUrl"]))
			if !ok || text == "" {
				continue
			}
			label := text
			if title != "" {
				label = title + ":" + text
			}
			if _, exists := out[image.key]; !exists {
				out[image.key] = label
			}
		}
	}
	return out
}

func saleSpecs(ware map[string]any) []SaleSpec {
	color, _ := ware["colorSizeVO"].(map[string]any)
	list, _ := color["colorSizeList"].([]any)
	var specs []SaleSpec
	for _, raw := range list {
		group, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		name := cleanText(stringValue(group["title"]))
		if name == "" {
			continue
		}
		var values []string
		buttons, _ := group["buttons"].([]any)
		for _, buttonRaw := range buttons {
			button, ok := buttonRaw.(map[string]any)
			if !ok {
				continue
			}
			text := cleanText(firstNonEmpty(stringValue(button["text"]), stringValue(button["name"])))
			if text != "" {
				values = appendUnique(values, text)
			}
		}
		if len(values) > 0 {
			specs = append(specs, SaleSpec{Name: name, Values: values})
		}
	}
	return specs
}

func attributes(ware map[string]any) []Attribute {
	raw, _ := ware["productAttributeVO"].(map[string]any)
	var items []Attribute
	for _, key := range []string{"attributes", "coreAttributes"} {
		list, _ := raw[key].([]any)
		for _, entry := range list {
			obj, ok := entry.(map[string]any)
			if !ok {
				continue
			}
			name := cleanText(firstNonEmpty(stringValue(obj["labelName"]), stringValue(obj["name"]), stringValue(obj["attrName"])))
			value := cleanText(firstNonEmpty(stringValue(obj["labelValue"]), stringValue(obj["value"]), stringValue(obj["attrValue"])))
			if name == "" || value == "" || strings.Contains(value, "<") {
				continue
			}
			items = appendUniqueAttr(items, Attribute{Name: name, Value: truncateRunes(value, 200)})
		}
	}
	return items
}

func detailImages(graphic map[string]any) []Image {
	var chunks []string
	if content := stringValue(graphic["graphicContent"]); content != "" {
		chunks = append(chunks, content)
	}
	if list, ok := graphic["graphicInfoList"].([]any); ok {
		for _, raw := range list {
			item, ok := raw.(map[string]any)
			if !ok {
				continue
			}
			if htmlText := stringValue(item["html"]); htmlText != "" {
				chunks = append(chunks, htmlText)
			}
		}
	}
	var images []Image
	for _, chunk := range chunks {
		for _, match := range htmlImage.FindAllStringSubmatch(chunk, -1) {
			if image, ok := canonicalImage(match[1]); ok {
				images = append(images, Image{Role: "detail", SourceURL: image.url})
			}
		}
		for _, match := range cssImage.FindAllStringSubmatch(chunk, -1) {
			if image, ok := canonicalImage(match[1]); ok {
				images = append(images, Image{Role: "detail", SourceURL: image.url})
			}
		}
	}
	return images
}

type canonical struct {
	url string
	key string
}

func canonicalImage(raw string) (canonical, bool) {
	raw = html.UnescapeString(strings.TrimSpace(raw))
	raw = strings.ReplaceAll(raw, "\\/", "/")
	match := jfsPattern.FindStringSubmatch(raw)
	if match == nil {
		return canonical{}, false
	}
	key := strings.ToLower(match[1])
	if decorative(key) {
		return canonical{}, false
	}
	// n0 会叠上京东展示水印；imgzone 是商家原图。
	return canonical{url: "https://img14.360buyimg.com/imgzone/" + match[1], key: key}, true
}

func imageKey(raw string) string {
	image, ok := canonicalImage(raw)
	if !ok {
		return ""
	}
	return image.key
}

func decorative(key string) bool {
	for _, part := range []string{"/icon", "sprite", "blank", "placeholder", "loading"} {
		if strings.Contains(key, part) {
			return true
		}
	}
	return false
}

func dedupeImages(images []Image) []Image {
	seen := map[string]bool{}
	var gallery, detail []Image
	for _, image := range images {
		key := imageKey(image.SourceURL)
		if key == "" || seen[key] {
			continue
		}
		seen[key] = true
		if image.Role == "detail" {
			if len(detail) >= maxDetailImages {
				continue
			}
			detail = append(detail, image)
			continue
		}
		if len(gallery) >= maxGalleryImages {
			continue
		}
		gallery = append(gallery, image)
	}
	out := make([]Image, 0, len(gallery)+len(detail))
	for i, image := range gallery {
		image.Role = "gallery"
		image.Index = i + 1
		out = append(out, image)
	}
	for i, image := range detail {
		image.Role = "detail"
		image.Index = i + 1
		out = append(out, image)
	}
	return out
}

func firstString(root map[string]any, keys ...string) string {
	wanted := map[string]bool{}
	for _, key := range keys {
		wanted[strings.ToLower(key)] = true
	}
	var found string
	var walk func(any)
	walk = func(value any) {
		if found != "" {
			return
		}
		switch item := value.(type) {
		case map[string]any:
			for key, child := range item {
				if wanted[strings.ToLower(key)] {
					if text := cleanText(stringValue(child)); text != "" && !strings.Contains(text, "://") {
						found = text
						return
					}
				}
			}
			for _, child := range item {
				walk(child)
				if found != "" {
					return
				}
			}
		case []any:
			for _, child := range item {
				walk(child)
				if found != "" {
					return
				}
			}
		}
	}
	walk(root)
	return found
}

func stringValue(value any) string {
	switch item := value.(type) {
	case string:
		return item
	case float64:
		if item == float64(int64(item)) {
			return strconv.FormatInt(int64(item), 10)
		}
	}
	return ""
}

func cleanText(value string) string {
	value = html.UnescapeString(strings.TrimSpace(value))
	return strings.Join(strings.Fields(value), " ")
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func appendUnique(values []string, value string) []string {
	for _, existing := range values {
		if existing == value {
			return values
		}
	}
	return append(values, value)
}

func appendUniqueAttr(items []Attribute, item Attribute) []Attribute {
	for _, existing := range items {
		if existing.Name == item.Name && existing.Value == item.Value {
			return items
		}
	}
	return append(items, item)
}

func truncateRunes(value string, limit int) string {
	runes := []rune(value)
	if len(runes) <= limit {
		return value
	}
	return string(runes[:limit])
}
