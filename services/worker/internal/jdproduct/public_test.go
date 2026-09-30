package jdproduct

import (
	"context"
	"os"
	"testing"
)

func TestParsePublicWareAndDetailPages(t *testing.T) {
	ware := `<script>window._itemOnly = ({"item":{"image":["jfs/t1/1/black.jpg","jfs/t1/2/side.jpg"],"saleProp":{"1":"颜色"},"salePropSeq":{"1":["黑色","白色"]},"newColorSize":[{"color":"黑色","imagePath":"jfs/t1/1/black.jpg","skuId":"10041037225218"},{"color":"白色","imagePath":"jfs/t1/3/white.jpg","skuId":"10041037225219"}],"skuId":"10041037225218","skuName":"短标题..."}});</script><script>"skuName":"完整的商品标题 黑色款"</script>`
	graphic := `<input id="wareGuigNew" value='{"propGroups":[{"atts":[{"attName":"品牌","vals":["JRAUDIO"]}]}]}'/>
<span class="title">包装清单</span><div class="content-block">保修卡，说明书</div>
.ssd{background-image:url(//img30.360buyimg.com/sku/jfs/t1/4/detail.jpg.dpg)}
<img src="https://img10.360buyimg.com/imagetools/jfs/t1/9/icon.png">`
	var product Product
	applyWareView(&product, ware)
	applyGraphext(&product, graphic)
	product.Images = dedupeImages(product.Images)
	if product.Title != "完整的商品标题 黑色款" {
		t.Fatalf("title %q", product.Title)
	}
	if len(product.SaleSpecs) != 1 || product.SaleSpecs[0].Name != "颜色" || len(product.SaleSpecs[0].Values) != 2 {
		t.Fatalf("specs %+v", product.SaleSpecs)
	}
	if len(product.Attributes) != 2 || product.Attributes[1].Value != "保修卡，说明书" {
		t.Fatalf("attrs %+v", product.Attributes)
	}
	if len(product.Images) != 4 {
		t.Fatalf("images %+v", product.Images)
	}
	if product.Images[0].Role != "gallery" || product.Images[0].Spec != "颜色:黑色" {
		t.Fatalf("first %+v", product.Images[0])
	}
	if product.Images[2].SourceURL != "https://img14.360buyimg.com/imgzone/jfs/t1/3/white.jpg" {
		t.Fatalf("white %+v", product.Images[2])
	}
	if product.Images[3].Role != "detail" {
		t.Fatalf("detail %+v", product.Images[3])
	}
}

func TestLoadPublicLive(t *testing.T) {
	if os.Getenv("JD_LIVE") != "1" {
		t.Skip("set JD_LIVE=1 to read a real Jingdong product page")
	}
	product, err := loadPublicProduct(context.Background(), "10041037225218")
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("title=%q images=%d specs=%d attrs=%d", product.Title, len(product.Images), len(product.SaleSpecs), len(product.Attributes))
	if len(product.Images) == 0 || product.Title == "" {
		t.Fatalf("empty product %+v", product)
	}
}
