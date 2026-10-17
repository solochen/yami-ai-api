package main

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"sync"
)

//go:embed ecom_factory_catalog.json
var ecomFactoryCatalogJSON []byte

// 电商图片工厂目录：模块/套装/批量维度/提示词模板的唯一事实源。
// API 侧 catalog 接口与前端计划预览共用同一份 JSON，保证提示词逐字一致。
type ecomCatalog struct {
	Version      int            `json:"version"`
	Limits       ecomLimits     `json:"limits"`
	Platforms    []string       `json:"platforms"`
	Languages    []string       `json:"languages"`
	VisualStyles []string       `json:"visualStyles"`
	Ratios       []string       `json:"ratios"`
	Resolutions  []string       `json:"resolutions"`
	SizeMap      map[string]map[string]string `json:"sizeMap"`
	Modules      []ecomModule   `json:"modules"`
	Suites       []ecomSuite    `json:"suites"`
	Batch        ecomBatchOpts  `json:"batch"`
	Templates    ecomTemplates  `json:"templates"`
	Copy         ecomCopyTexts  `json:"copy"`
}

type ecomCopyTexts struct {
	StatusLabels     map[string]string `json:"statusLabels"`
	BatchStatusLabels map[string]string `json:"batchStatusLabels"`
	EmptyResult      string            `json:"emptyResult"`
	NeedInput        string            `json:"needInput"`
	NeedBatchPrompt  string            `json:"needBatchPrompt"`
	ModelNotLoaded   string            `json:"modelNotLoaded"`
}

type ecomLimits struct {
	MaxProductImages   int `json:"maxProductImages"`
	MaxReferenceImages int `json:"maxReferenceImages"`
	MaxPlanItems       int `json:"maxPlanItems"`
	MaxCountPerModule  int `json:"maxCountPerModule"`
	MaxConcurrency     int `json:"maxConcurrency"`
	MaxBatchCount      int `json:"maxBatchCount"`
	QuickMaxReferences int `json:"quickMaxReferences"`
	DefaultMaxRefs     int `json:"defaultMaxReferences"`
}

type ecomModule struct {
	ID           string `json:"id"`
	Label        string `json:"label"`
	ShortLabel   string `json:"shortLabel"`
	Channel      string `json:"channel"`
	DefaultRatio string `json:"defaultRatio"`
	Goal         string `json:"goal"`
	Prompt       string `json:"prompt"`
	Accent       string `json:"accent"`
}

type ecomSuite struct {
	ID          string   `json:"id"`
	Label       string   `json:"label"`
	Description string   `json:"description"`
	ModuleIds   []string `json:"moduleIds"`
}

type ecomBatchOpts struct {
	Strategies       []string          `json:"strategies"`
	Similarities     []string          `json:"similarities"`
	Variations       []string          `json:"variations"`
	CountOptions     []int             `json:"countOptions"`
	DefaultKeepKeys  []string          `json:"defaultKeepKeys"`
	DefaultChangeKeys []string          `json:"defaultChangeKeys"`
	KeepKeys         []ecomBatchKey    `json:"keepKeys"`
	ChangeKeys       []ecomBatchKey    `json:"changeKeys"`
}

type ecomBatchKey struct {
	Key   string `json:"key"`
	Label string `json:"label"`
}

type ecomTemplates struct {
	Normal  ecomLineTemplate `json:"normal"`
	Batch   ecomLineTemplate `json:"batch"`
	Refine  ecomRefineTemplate `json:"refine"`
	Reverse ecomReverseTemplate `json:"reverse"`
}

type ecomLineTemplate struct {
	Lines []ecomTemplateLine `json:"lines"`
}

type ecomTemplateLine struct {
	When string `json:"when,omitempty"`
	Text string `json:"text"`
}

type ecomRefineTemplate struct {
	Prefix             string `json:"prefix"`
	DefaultInstruction string `json:"defaultInstruction"`
}

type ecomReverseTemplate struct {
	Prompt string `json:"prompt"`
}

var (
	ecomCatalogOnce sync.Once
	ecomCatalogPtr  *ecomCatalog
	ecomCatalogErr  error
)

func loadEcomCatalog() (*ecomCatalog, error) {
	ecomCatalogOnce.Do(func() {
		cat := &ecomCatalog{}
		if err := json.Unmarshal(ecomFactoryCatalogJSON, cat); err != nil {
			ecomCatalogErr = fmt.Errorf("解析电商图片工厂目录失败: %w", err)
			return
		}
		ecomCatalogPtr = cat
	})
	return ecomCatalogPtr, ecomCatalogErr
}

func (c *ecomCatalog) ModuleByID(id string) (ecomModule, bool) {
	for _, m := range c.Modules {
		if m.ID == id {
			return m, true
		}
	}
	return ecomModule{}, false
}

func (c *ecomCatalog) SuiteByID(id string) (ecomSuite, bool) {
	for _, s := range c.Suites {
		if s.ID == id {
			return s, true
		}
	}
	return ecomSuite{}, false
}

func (c *ecomCatalog) IsBatchKey(key string, keep bool) bool {
	list := c.Batch.ChangeKeys
	if keep {
		list = c.Batch.KeepKeys
	}
	for _, k := range list {
		if k.Key == key {
			return true
		}
	}
	return false
}

// resolveEcomSize 复刻参考站尺寸映射：优先精确 (resolution, ratio)，缺档回退 1K 表，
// 已是 WxH 形式则透传。
func (c *ecomCatalog) resolveEcomSize(ratio, resolution string) string {
	if ratio != "" && isPixelSize(ratio) {
		return ratio
	}
	tier := resolution
	if _, ok := c.SizeMap[tier]; !ok {
		tier = "1K"
	}
	if size, ok := c.SizeMap[tier][ratio]; ok {
		return size
	}
	if size, ok := c.SizeMap["1K"][ratio]; ok {
		return size
	}
	return "1024x1024"
}

func isPixelSize(s string) bool {
	var w, h int
	_, err := fmt.Sscanf(s, "%dx%d", &w, &h)
	return err == nil && w > 0 && h > 0
}
