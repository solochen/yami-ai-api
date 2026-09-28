package service

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
)

func (s *PaymentService) createAlipayPagePay(cfg PaymentProviderConfig, orderNo string, amount float64, expiresAt time.Time) (string, error) {
	if cfg.Currency != "CNY" {
		return "", errors.New("支付宝仅支持收款币种 CNY")
	}
	privateKey, err := parseRSAPrivateKey(cfg.AlipayPrivateKey)
	if err != nil {
		return "", errors.New("支付宝应用私钥无效")
	}
	total, err := cnyAmountString(amount)
	if err != nil {
		return "", err
	}
	biz, err := json.Marshal(map[string]string{
		"out_trade_no":    orderNo,
		"product_code":    "FAST_INSTANT_TRADE_PAY",
		"subject":         paymentSubject(cfg.ProductName),
		"timeout_express": alipayTimeoutExpress(expiresAt),
		"total_amount":    total,
	})
	if err != nil {
		return "", err
	}
	params := map[string]string{
		"app_id":      cfg.AlipayAppID,
		"biz_content": string(biz),
		"charset":     "utf-8",
		"method":      "alipay.trade.page.pay",
		"notify_url":  paymentNotifyURL(cfg.NotifyBaseURL, "/api/payment/webhooks/alipay"),
		"return_url":  replacePaymentURL(cfg.SuccessURL, orderNo),
		"sign_type":   "RSA2",
		"timestamp":   chinaNow().Format("2006-01-02 15:04:05"),
		"version":     "1.0",
	}
	signature, err := signRSA2(alipaySignContent(params), privateKey)
	if err != nil {
		return "", err
	}
	params["sign"] = signature
	query := url.Values{}
	for key, value := range params {
		query.Set(key, value)
	}
	gateway := s.alipayGateway
	if cfg.AlipaySandbox {
		gateway = s.alipaySandboxGateway
	}
	if gateway == "" {
		gateway = "https://openapi.alipay.com/gateway.do"
	}
	return strings.TrimRight(gateway, "?") + "?" + query.Encode(), nil
}

func (s *PaymentService) CompleteAlipayWebhook(ctx context.Context, raw []byte) (*PaymentCompletion, error) {
	cfg, err := s.ProviderConfig(ctx)
	if err != nil {
		return nil, err
	}
	if !cfg.AlipayReady() {
		return nil, errors.New("支付宝尚未完整配置")
	}
	values, err := url.ParseQuery(string(raw))
	if err != nil {
		return nil, errors.New("支付宝回调格式错误")
	}
	params := map[string]string{}
	for key, items := range values {
		if len(items) > 0 {
			params[key] = items[0]
		}
	}
	publicKey, err := parseRSAPublicKey(cfg.AlipayPublicKey)
	if err != nil {
		return nil, errors.New("支付宝公钥无效")
	}
	if err := verifyRSA2(alipaySignContent(params), params["sign"], publicKey); err != nil {
		return nil, err
	}
	if params["app_id"] != cfg.AlipayAppID {
		return nil, errors.New("支付宝应用不匹配")
	}
	switch params["trade_status"] {
	case "TRADE_SUCCESS", "TRADE_FINISHED":
	default:
		return nil, nil
	}
	amount, err := strconv.ParseFloat(params["total_amount"], 64)
	if err != nil {
		return nil, errors.New("支付宝金额无效")
	}
	return s.completeOrder(ctx, params["out_trade_no"], "alipay", params["trade_no"], amount, "CNY", paymentCallbackDigest(params["sign"]))
}

func alipaySignContent(params map[string]string) string {
	keys := make([]string, 0, len(params))
	for key, value := range params {
		if key == "sign" || key == "sign_type" || strings.TrimSpace(value) == "" {
			continue
		}
		keys = append(keys, key)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, key+"="+params[key])
	}
	return strings.Join(parts, "&")
}

func alipayTimeoutExpress(expiresAt time.Time) string {
	minutes := int(math.Ceil(time.Until(expiresAt).Minutes()))
	if minutes < 1 {
		minutes = 1
	}
	if minutes > 15*24*60 {
		minutes = 15 * 24 * 60
	}
	return strconv.Itoa(minutes) + "m"
}

func cnyAmountString(amount float64) (string, error) {
	fen, err := cnyFen(amount)
	if err != nil {
		return "", err
	}
	return strconv.FormatFloat(float64(fen)/100, 'f', 2, 64), nil
}

func cnyFen(amount float64) (int64, error) {
	fen := math.Round(amount * 100)
	if fen < 1 || math.Abs(amount*100-fen) > 0.001 {
		return 0, errors.New("支付金额需为最多两位小数的正数")
	}
	return int64(fen), nil
}

func paymentSubject(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		name = "算力充值"
	}
	runes := []rune(name)
	if len(runes) > 40 {
		name = string(runes[:40])
	}
	return name
}

func paymentNotifyURL(base, path string) string {
	return strings.TrimRight(strings.TrimSpace(base), "/") + path
}

func chinaNow() time.Time {
	return time.Now().In(time.FixedZone("CST", 8*3600))
}
