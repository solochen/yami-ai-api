package service

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestAlipayPagePaySignsAmountAndCNYGate(t *testing.T) {
	privatePEM, publicPEM := testPaymentKeyPEM(t)
	cfg := PaymentProviderConfig{
		Enabled: true, Currency: "CNY", ProductName: "算力充值", AlipayEnabled: true,
		AlipayAppID: "2021000000000000", AlipayPrivateKey: privatePEM, AlipayPublicKey: publicPEM,
		NotifyBaseURL: "https://api.example.com", SuccessURL: "https://example.com/app/wallet?payment=success&order={order_no}",
	}
	if !cfg.AlipayReady() {
		t.Fatal("complete Alipay config should be ready")
	}
	cfg.Currency = "USD"
	if cfg.AlipayReady() {
		t.Fatal("Alipay must require CNY")
	}
	cfg.Currency = "CNY"
	svc := &PaymentService{alipayGateway: "https://openapi.alipay.com/gateway.do"}
	checkout, err := svc.createAlipayPagePay(cfg, "ord_test", 10, time.Now().Add(30*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := url.Parse(checkout)
	if err != nil {
		t.Fatal(err)
	}
	query := parsed.Query()
	if query.Get("method") != "alipay.trade.page.pay" || !strings.Contains(query.Get("biz_content"), `"total_amount":"10.00"`) {
		t.Fatalf("unexpected alipay checkout: %s", checkout)
	}
	publicKey, err := parseRSAPublicKey(publicPEM)
	if err != nil {
		t.Fatal(err)
	}
	params := map[string]string{}
	for key, values := range query {
		params[key] = values[0]
	}
	if err := verifyRSA2(alipaySignContent(params), params["sign"], publicKey); err != nil {
		t.Fatal(err)
	}
}

func TestWechatResourceDecryptAndFen(t *testing.T) {
	fen, err := cnyFen(10)
	if err != nil || fen != 1000 {
		t.Fatalf("cnyFen(10)=%d %v", fen, err)
	}
	if _, err := cnyFen(10.001); err == nil {
		t.Fatal("three decimal places should be rejected")
	}
	key := strings.Repeat("k", 32)
	block, err := aes.NewCipher([]byte(key))
	if err != nil {
		t.Fatal(err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		t.Fatal(err)
	}
	nonce := []byte("0123456789ab")
	sealed := gcm.Seal(nil, nonce, []byte(`{"trade_state":"SUCCESS"}`), []byte("transaction"))
	plain, err := decryptWechatResource(key, string(nonce), "transaction", base64.StdEncoding.EncodeToString(sealed))
	if err != nil || string(plain) != `{"trade_state":"SUCCESS"}` {
		t.Fatalf("decrypt=%s %v", plain, err)
	}
}

func TestChinaChannelsStayIndependent(t *testing.T) {
	privatePEM, publicPEM := testPaymentKeyPEM(t)
	cfg := PaymentProviderConfig{
		Enabled: true, Provider: "stripe", Currency: "CNY",
		StripeSecretKey: "sk_test_example", StripeWebhookSecret: "whsec_example",
		SuccessURL: "https://example.com/success", CancelURL: "https://example.com/cancel",
		AlipayEnabled: true, AlipayAppID: "2021000000000000", AlipayPrivateKey: privatePEM, AlipayPublicKey: publicPEM,
		NotifyBaseURL: "https://api.example.com",
		WechatEnabled: true, WechatAppID: "wx123", WechatMchID: "1900000001", WechatCertSerial: "SERIAL",
		WechatAPIv3Key: strings.Repeat("k", 32), WechatPrivateKey: privatePEM,
	}
	channels := cfg.ReadyChannels()
	if strings.Join(channels, ",") != "stripe,alipay,wechat" {
		t.Fatalf("channels=%v", channels)
	}
}

func testPaymentKeyPEM(t *testing.T) (string, string) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	privateDER, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	publicDER, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	if err != nil {
		t.Fatal(err)
	}
	privatePEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: privateDER})
	publicPEM := pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: publicDER})
	return string(privatePEM), string(publicPEM)
}
