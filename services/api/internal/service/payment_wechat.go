package service

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/skip2/go-qrcode"
)

func (s *PaymentService) createWechatNativeOrder(ctx context.Context, cfg PaymentProviderConfig, orderNo string, amount float64, expiresAt time.Time) (string, error) {
	if cfg.Currency != "CNY" {
		return "", errors.New("微信支付仅支持收款币种 CNY")
	}
	fen, err := cnyFen(amount)
	if err != nil {
		return "", err
	}
	privateKey, err := parseRSAPrivateKey(cfg.WechatPrivateKey)
	if err != nil {
		return "", errors.New("微信商户私钥无效")
	}
	body, err := json.Marshal(map[string]interface{}{
		"appid":        cfg.WechatAppID,
		"mchid":        cfg.WechatMchID,
		"description":  paymentSubject(cfg.ProductName),
		"out_trade_no": orderNo,
		"notify_url":   paymentNotifyURL(cfg.NotifyBaseURL, "/api/payment/webhooks/wechat"),
		"time_expire":  expiresAt.In(time.FixedZone("CST", 8*3600)).Format(time.RFC3339),
		"amount":       map[string]interface{}{"total": fen, "currency": "CNY"},
	})
	if err != nil {
		return "", err
	}
	path := "/v3/pay/transactions/native"
	raw, status, err := s.providerRequest(ctx, http.MethodPost, strings.TrimRight(s.wechatAPIBase, "/")+path, map[string]string{
		"Accept":        "application/json",
		"Content-Type":  "application/json",
		"Authorization": wechatAuthorization(cfg.WechatMchID, cfg.WechatCertSerial, http.MethodPost, path, string(body), privateKey),
	}, body)
	if err != nil {
		return "", err
	}
	if status < 200 || status >= 300 {
		return "", paymentProviderAPIError("微信支付", status, raw)
	}
	var response struct {
		CodeURL string `json:"code_url"`
	}
	if json.Unmarshal(raw, &response) != nil || strings.TrimSpace(response.CodeURL) == "" {
		return "", errors.New("微信支付未返回收款码")
	}
	return response.CodeURL, nil
}

func (s *PaymentService) CompleteWechatWebhook(ctx context.Context, raw []byte, headers map[string]string) (*PaymentCompletion, error) {
	cfg, err := s.ProviderConfig(ctx)
	if err != nil {
		return nil, err
	}
	if !cfg.WechatReady() {
		return nil, errors.New("微信支付尚未完整配置")
	}
	timestamp := strings.TrimSpace(headers["Wechatpay-Timestamp"])
	nonce := strings.TrimSpace(headers["Wechatpay-Nonce"])
	signature := strings.TrimSpace(headers["Wechatpay-Signature"])
	serial := strings.TrimSpace(headers["Wechatpay-Serial"])
	if timestamp == "" || nonce == "" || signature == "" || serial == "" {
		return nil, errors.New("微信支付回调头不完整")
	}
	ts, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil || mathAbs(time.Now().Unix()-ts) > 300 {
		return nil, errors.New("微信支付回调时间戳无效或已过期")
	}
	platformKey, err := s.wechatPlatformKey(ctx, cfg, serial)
	if err != nil {
		return nil, err
	}
	message := timestamp + "\n" + nonce + "\n" + string(raw) + "\n"
	if err := verifyRSA2(message, signature, platformKey); err != nil {
		if refreshErr := s.refreshWechatCertificates(ctx, cfg); refreshErr == nil {
			if refreshed, keyErr := s.cachedWechatKey(serial); keyErr == nil {
				err = verifyRSA2(message, signature, refreshed)
			}
		}
		if err != nil {
			return nil, errors.New("微信支付回调签名校验失败")
		}
	}
	if err := s.rememberWechatNonce(serial+":"+nonce, time.Unix(ts, 0)); err != nil {
		return nil, err
	}
	var envelope struct {
		Resource struct {
			Algorithm      string `json:"algorithm"`
			Ciphertext     string `json:"ciphertext"`
			Nonce          string `json:"nonce"`
			AssociatedData string `json:"associated_data"`
		} `json:"resource"`
	}
	if json.Unmarshal(raw, &envelope) != nil {
		return nil, errors.New("微信支付回调格式错误")
	}
	if envelope.Resource.Algorithm != "" && envelope.Resource.Algorithm != "AEAD_AES_256_GCM" {
		return nil, errors.New("微信支付回调加密算法不受支持")
	}
	plain, err := decryptWechatResource(cfg.WechatAPIv3Key, envelope.Resource.Nonce, envelope.Resource.AssociatedData, envelope.Resource.Ciphertext)
	if err != nil {
		return nil, errors.New("微信支付回调解密失败")
	}
	var transaction struct {
		AppID         string `json:"appid"`
		MchID         string `json:"mchid"`
		OutTradeNo    string `json:"out_trade_no"`
		TransactionID string `json:"transaction_id"`
		TradeState    string `json:"trade_state"`
		Amount        struct {
			Total    int64  `json:"total"`
			Currency string `json:"currency"`
		} `json:"amount"`
	}
	if json.Unmarshal(plain, &transaction) != nil {
		return nil, errors.New("微信支付交易数据无效")
	}
	if transaction.AppID != cfg.WechatAppID || transaction.MchID != cfg.WechatMchID {
		return nil, errors.New("微信支付商户不匹配")
	}
	if transaction.TradeState != "SUCCESS" {
		return nil, nil
	}
	if !strings.EqualFold(transaction.Amount.Currency, "CNY") || transaction.Amount.Total < 1 {
		return nil, errors.New("微信支付金额无效")
	}
	return s.completeOrder(ctx, transaction.OutTradeNo, "wechat", transaction.TransactionID, float64(transaction.Amount.Total)/100, "CNY", paymentCallbackDigest(serial+":"+nonce))
}

func wechatAuthorization(mchid, serial, method, path, body string, key *rsa.PrivateKey) string {
	nonceBytes := make([]byte, 16)
	_, _ = rand.Read(nonceBytes)
	nonce := hex.EncodeToString(nonceBytes)
	timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	signature, err := signRSA2(method+"\n"+path+"\n"+timestamp+"\n"+nonce+"\n"+body+"\n", key)
	if err != nil {
		return ""
	}
	return fmt.Sprintf(`WECHATPAY2-SHA256-RSA2048 mchid="%s",nonce_str="%s",signature="%s",timestamp="%s",serial_no="%s"`, mchid, nonce, signature, timestamp, serial)
}

func decryptWechatResource(apiV3Key, nonce, associated, ciphertext string) ([]byte, error) {
	if len(apiV3Key) != 32 {
		return nil, errors.New("APIv3 密钥长度必须为 32")
	}
	raw, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher([]byte(apiV3Key))
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return gcm.Open(nil, []byte(nonce), raw, []byte(associated))
}

func (s *PaymentService) rememberWechatNonce(nonce string, seenAt time.Time) error {
	s.wechatMu.Lock()
	defer s.wechatMu.Unlock()
	if s.wechatNonces == nil {
		s.wechatNonces = map[string]time.Time{}
	}
	for key, seen := range s.wechatNonces {
		if seenAt.Sub(seen) > 10*time.Minute {
			delete(s.wechatNonces, key)
		}
	}
	if _, exists := s.wechatNonces[nonce]; exists {
		return errors.New("微信支付回调重复")
	}
	s.wechatNonces[nonce] = seenAt
	return nil
}

func (s *PaymentService) wechatPlatformKey(ctx context.Context, cfg PaymentProviderConfig, serial string) (*rsa.PublicKey, error) {
	if key, err := s.cachedWechatKey(serial); err == nil {
		return key, nil
	}
	if err := s.refreshWechatCertificates(ctx, cfg); err != nil {
		return nil, err
	}
	return s.cachedWechatKey(serial)
}

func (s *PaymentService) cachedWechatKey(serial string) (*rsa.PublicKey, error) {
	s.wechatMu.Lock()
	defer s.wechatMu.Unlock()
	key := s.wechatCerts[serial]
	if key == nil {
		return nil, errors.New("未找到微信支付平台证书")
	}
	return key, nil
}

func (s *PaymentService) refreshWechatCertificates(ctx context.Context, cfg PaymentProviderConfig) error {
	privateKey, err := parseRSAPrivateKey(cfg.WechatPrivateKey)
	if err != nil {
		return err
	}
	path := "/v3/certificates"
	base := s.wechatAPIBase
	if base == "" {
		base = "https://api.mch.weixin.qq.com"
	}
	raw, status, err := s.providerRequest(ctx, http.MethodGet, strings.TrimRight(base, "/")+path, map[string]string{
		"Accept":        "application/json",
		"Authorization": wechatAuthorization(cfg.WechatMchID, cfg.WechatCertSerial, http.MethodGet, path, "", privateKey),
	}, nil)
	if err != nil {
		return err
	}
	if status < 200 || status >= 300 {
		return paymentProviderAPIError("微信支付", status, raw)
	}
	var response struct {
		Data []struct {
			SerialNo           string `json:"serial_no"`
			EncryptCertificate struct {
				Nonce          string `json:"nonce"`
				AssociatedData string `json:"associated_data"`
				Ciphertext     string `json:"ciphertext"`
			} `json:"encrypt_certificate"`
		} `json:"data"`
	}
	if json.Unmarshal(raw, &response) != nil {
		return errors.New("微信支付平台证书响应无效")
	}
	next := map[string]*rsa.PublicKey{}
	for _, item := range response.Data {
		plain, err := decryptWechatResource(cfg.WechatAPIv3Key, item.EncryptCertificate.Nonce, item.EncryptCertificate.AssociatedData, item.EncryptCertificate.Ciphertext)
		if err != nil {
			return err
		}
		publicKey, err := rsaPublicKeyFromCertificate(plain)
		if err != nil {
			return err
		}
		next[item.SerialNo] = publicKey
	}
	if len(next) == 0 {
		return errors.New("微信支付未返回平台证书")
	}
	s.wechatMu.Lock()
	if s.wechatCerts == nil {
		s.wechatCerts = map[string]*rsa.PublicKey{}
	}
	for serial, key := range next {
		s.wechatCerts[serial] = key
	}
	s.wechatMu.Unlock()
	return nil
}

func rsaPublicKeyFromCertificate(plain []byte) (*rsa.PublicKey, error) {
	der := plain
	if block, _ := pem.Decode(plain); block != nil {
		der = block.Bytes
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		return nil, err
	}
	publicKey, ok := cert.PublicKey.(*rsa.PublicKey)
	if !ok {
		return nil, errors.New("微信支付平台证书不是 RSA 密钥")
	}
	return publicKey, nil
}

func paymentQRImage(content string) string {
	png, err := qrcode.Encode(content, qrcode.Medium, 256)
	if err != nil || len(png) == 0 {
		return ""
	}
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(png)
}

func mathAbs(value int64) int64 {
	if value < 0 {
		return -value
	}
	return value
}
