package jdproduct

import (
	"context"
	"os"
	"testing"
	"time"
)

func TestCaptureLiveJDProduct(t *testing.T) {
	if os.Getenv("JD_LIVE") != "1" {
		t.Skip("set JD_LIVE=1 to read a real Jingdong product page")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	ware, graphic, err := Capture(ctx, "https://item.jd.com/10041037225218.html")
	if err != nil {
		t.Fatal(err)
	}
	product, err := Parse(ware, graphic)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("title=%q sku=%q gallery/detail images=%d specs=%d attrs=%d ware=%d graphic=%d", product.Title, product.SKU, len(product.Images), len(product.SaleSpecs), len(product.Attributes), len(ware), len(graphic))
	if len(product.Images) == 0 {
		t.Fatalf("no images, ware head: %.300s", ware)
	}
}
