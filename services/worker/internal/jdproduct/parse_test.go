package jdproduct

import "testing"

func TestParseProductImagesSpecsAndDetail(t *testing.T) {
	ware := []byte(`{
		"code": 0,
		"data": {
			"pageConfigVO": {"skuid": "10041037225218", "skuName": "测试保温杯 500ml"},
			"mainImageVO": {
				"mainImageArea": {"imageUrl": "jfs/t1/1/black.jpg"},
				"carouselArea": [
					{"imageUrl": "jfs/t1/1/black.jpg", "siteType": "1"},
					{"imageUrl": "//img10.360buyimg.com/n5/s800x800_jfs/t1/2/white.jpg", "siteType": "1"},
					{"imageUrl": "jfs/t1/9/icon.png", "siteType": "1"}
				]
			},
			"colorSizeVO": {
				"colorSizeList": [{
					"title": "颜色",
					"buttons": [
						{"text": "黑色", "skuId": "10041037225218", "imageUrl": "jfs/t1/1/black.jpg"},
						{"text": "白色", "skuId": "10041037225219", "imageUrl": "jfs/t1/2/white.jpg"},
						{"text": "黑色", "skuId": "10041037225220", "imageUrl": "jfs/t1/1/black.jpg"}
					]
				}]
			},
			"productAttributeVO": {
				"attributes": [{"labelName": "材质", "labelValue": "不锈钢"}],
				"coreAttributes": [{"labelName": "容量", "labelValue": "500ml"}]
			}
		}
	}`)
	graphic := []byte(`{
		"code": 0,
		"data": {
			"wareQD": "说明书 x1",
			"graphicContent": "<img data-lazyload=\"//img14.360buyimg.com/n1/s800x800_jfs/t1/3/detail.jpg\"><img src=\"https://img14.360buyimg.com/n0/jfs/t1/1/black.jpg\"><div style=\"background-image:url('jfs/t1/4/scene.png')\"></div>"
		}
	}`)
	product, err := Parse(ware, graphic)
	if err != nil {
		t.Fatal(err)
	}
	if product.SKU != "10041037225218" || product.Title != "测试保温杯 500ml" {
		t.Fatalf("identity = %+v", product)
	}
	if len(product.SaleSpecs) != 1 || product.SaleSpecs[0].Name != "颜色" || len(product.SaleSpecs[0].Values) != 2 {
		t.Fatalf("specs = %+v", product.SaleSpecs)
	}
	if len(product.Attributes) != 3 || product.Attributes[2].Name != "包装清单" {
		t.Fatalf("attributes = %+v", product.Attributes)
	}
	if len(product.Images) != 4 {
		t.Fatalf("images = %+v", product.Images)
	}
	if product.Images[0].Role != "gallery" || product.Images[0].Spec != "颜色:黑色" || product.Images[0].Index != 1 {
		t.Fatalf("first image = %+v", product.Images[0])
	}
	if product.Images[1].SourceURL != "https://img14.360buyimg.com/imgzone/jfs/t1/2/white.jpg" {
		t.Fatalf("white image = %+v", product.Images[1])
	}
	if product.Images[2].Role != "detail" || product.Images[3].Role != "detail" {
		t.Fatalf("detail images = %+v", product.Images[2:])
	}
}

func TestParseRejectsEmptyAndBlockedPayloads(t *testing.T) {
	product, err := Parse([]byte(`{"echo":"no access"}`), nil)
	if err != nil {
		t.Fatal(err)
	}
	if product.Title != "" || len(product.Images) != 0 {
		t.Fatalf("unexpected product %+v", product)
	}
	product, err = Parse(nil, []byte(`not-json`))
	if err != nil || len(product.Images) != 0 {
		t.Fatalf("invalid graphic should be ignored: %+v %v", product, err)
	}
}
