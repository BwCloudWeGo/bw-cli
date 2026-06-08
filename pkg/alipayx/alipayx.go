// alipayx 包用适配框架的配置和窄接口封装 github.com/smartwalle/alipay/v3，
// 提供支付、通知和退款辅助能力。
package alipayx

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"

	alipay "github.com/smartwalle/alipay/v3"
)

const (
	// ProductCodePagePay 是支付宝电脑网站支付使用的产品码。
	ProductCodePagePay = "FAST_INSTANT_TRADE_PAY"
	// ProductCodeWapPay 是支付宝手机网站支付使用的产品码。
	ProductCodeWapPay = "QUICK_WAP_WAY"
	// ProductCodeAppPay 是支付宝 App 支付使用的产品码。
	ProductCodeAppPay = "QUICK_MSECURITY_PAY"
)

// Config 包含支付宝应用凭证和默认回调配置。
type Config struct {
	AppID                   string `mapstructure:"app_id" yaml:"app_id"`
	PrivateKey              string `mapstructure:"private_key" yaml:"private_key"`
	AlipayPublicKey         string `mapstructure:"alipay_public_key" yaml:"alipay_public_key"`
	Production              bool   `mapstructure:"production" yaml:"production"`
	NotifyURL               string `mapstructure:"notify_url" yaml:"notify_url"`
	ReturnURL               string `mapstructure:"return_url" yaml:"return_url"`
	EncryptKey              string `mapstructure:"encrypt_key" yaml:"encrypt_key"`
	AppCertPublicKeyPath    string `mapstructure:"app_cert_public_key_path" yaml:"app_cert_public_key_path"`
	AlipayRootCertPath      string `mapstructure:"alipay_root_cert_path" yaml:"alipay_root_cert_path"`
	AlipayCertPublicKeyPath string `mapstructure:"alipay_cert_public_key_path" yaml:"alipay_cert_public_key_path"`
}

// DefaultConfig 返回本地开发可用的支付宝默认配置。
func DefaultConfig() Config {
	return Config{}
}

// Client 提供框架侧收敛后的支付宝支付接口。
type Client struct {
	cfg Config
	raw *alipay.Client
}

// PayRequest 描述一笔支付宝支付订单。
type PayRequest struct {
	OutTradeNo     string
	Subject        string
	TotalAmount    string
	Body           string
	NotifyURL      string
	ReturnURL      string
	PassbackParams string
	TimeoutExpress string
}

// RefundRequest 描述一笔支付宝退款请求。
type RefundRequest struct {
	OutTradeNo   string
	TradeNo      string
	RefundAmount string
	RefundReason string
	OutRequestNo string
}

// NewClient 创建已配置的 smartwalle 支付宝客户端。
func NewClient(cfg Config) (*Client, error) {
	cfg = normalizeConfig(cfg)
	if cfg.AppID == "" {
		return nil, errors.New("alipay app id is required")
	}
	if cfg.PrivateKey == "" {
		return nil, errors.New("alipay private key is required")
	}
	if cfg.AlipayPublicKey != "" && hasAnyCertPath(cfg) {
		return nil, errors.New("alipay public key and certificate paths cannot be used together")
	}
	if hasAnyCertPath(cfg) && !hasAllCertPaths(cfg) {
		return nil, errors.New("alipay certificate mode requires app, root and alipay cert paths")
	}

	raw, err := alipay.New(cfg.AppID, cfg.PrivateKey, cfg.Production)
	if err != nil {
		return nil, fmt.Errorf("create alipay client: %w", err)
	}
	if cfg.EncryptKey != "" {
		if err := raw.SetEncryptKey(cfg.EncryptKey); err != nil {
			return nil, fmt.Errorf("set alipay encrypt key: %w", err)
		}
	}
	if cfg.AlipayPublicKey != "" {
		if err := raw.LoadAliPayPublicKey(cfg.AlipayPublicKey); err != nil {
			return nil, fmt.Errorf("load alipay public key: %w", err)
		}
	}
	if hasAllCertPaths(cfg) {
		if err := raw.LoadAppCertPublicKeyFromFile(cfg.AppCertPublicKeyPath); err != nil {
			return nil, fmt.Errorf("load alipay app cert public key: %w", err)
		}
		if err := raw.LoadAliPayRootCertFromFile(cfg.AlipayRootCertPath); err != nil {
			return nil, fmt.Errorf("load alipay root cert: %w", err)
		}
		if err := raw.LoadAlipayCertPublicKeyFromFile(cfg.AlipayCertPublicKeyPath); err != nil {
			return nil, fmt.Errorf("load alipay cert public key: %w", err)
		}
	}

	return &Client{cfg: cfg, raw: raw}, nil
}

// Raw 返回底层 smartwalle 客户端，用于少见的支付宝 API。
func (c *Client) Raw() *alipay.Client {
	if c == nil {
		return nil
	}
	return c.raw
}

// PagePay 构建电脑网站支付跳转 URL。
func (c *Client) PagePay(req PayRequest) (*url.URL, error) {
	if err := c.validateClient(); err != nil {
		return nil, err
	}
	if err := validatePayRequest(req); err != nil {
		return nil, err
	}
	param := alipay.TradePagePay{Trade: c.trade(req, ProductCodePagePay)}
	return c.raw.TradePagePay(param)
}

// PagePayURL 以字符串形式构建电脑网站支付跳转 URL。
func (c *Client) PagePayURL(req PayRequest) (string, error) {
	payURL, err := c.PagePay(req)
	if err != nil {
		return "", err
	}
	return payURL.String(), nil
}

// WapPay 构建手机网站支付跳转 URL。
func (c *Client) WapPay(req PayRequest) (*url.URL, error) {
	if err := c.validateClient(); err != nil {
		return nil, err
	}
	if err := validatePayRequest(req); err != nil {
		return nil, err
	}
	param := alipay.TradeWapPay{Trade: c.trade(req, ProductCodeWapPay)}
	return c.raw.TradeWapPay(param)
}

// WapPayURL 以字符串形式构建手机网站支付跳转 URL。
func (c *Client) WapPayURL(req PayRequest) (string, error) {
	payURL, err := c.WapPay(req)
	if err != nil {
		return "", err
	}
	return payURL.String(), nil
}

// AppPay 构建移动端传给支付宝 SDK 的订单字符串。
func (c *Client) AppPay(req PayRequest) (string, error) {
	if err := c.validateClient(); err != nil {
		return "", err
	}
	if err := validatePayRequest(req); err != nil {
		return "", err
	}
	param := alipay.TradeAppPay{Trade: c.trade(req, ProductCodeAppPay)}
	return c.raw.TradeAppPay(param)
}

// DecodeNotification 验证异步通知，并转换为强类型载荷。
func (c *Client) DecodeNotification(ctx context.Context, values url.Values) (*alipay.Notification, error) {
	if c == nil || c.raw == nil {
		return nil, errors.New("alipay client is nil")
	}
	return c.raw.DecodeNotification(ctx, values)
}

// VerifyReturn 验证同步 return_url 跳转携带的签名 query/form 参数。
func (c *Client) VerifyReturn(ctx context.Context, values url.Values) error {
	if c == nil || c.raw == nil {
		return errors.New("alipay client is nil")
	}
	return c.raw.VerifySign(ctx, values)
}

// Refund 向支付宝提交同步退款请求。
func (c *Client) Refund(ctx context.Context, req RefundRequest) (*alipay.TradeRefundRsp, error) {
	if err := c.validateClient(); err != nil {
		return nil, err
	}
	if err := validateRefundRequest(req); err != nil {
		return nil, err
	}
	param := alipay.TradeRefund{
		OutTradeNo:   strings.TrimSpace(req.OutTradeNo),
		TradeNo:      strings.TrimSpace(req.TradeNo),
		RefundAmount: strings.TrimSpace(req.RefundAmount),
		RefundReason: strings.TrimSpace(req.RefundReason),
		OutRequestNo: strings.TrimSpace(req.OutRequestNo),
	}
	return c.raw.TradeRefund(ctx, param)
}

// RefundOK 提交退款请求，并在支付宝拒绝时返回错误。
func (c *Client) RefundOK(ctx context.Context, req RefundRequest) error {
	rsp, err := c.Refund(ctx, req)
	if err != nil {
		return err
	}
	return ensureRefundSuccess(rsp)
}

func (c *Client) validateClient() error {
	if c == nil || c.raw == nil {
		return errors.New("alipay client is nil")
	}
	return nil
}

func ensureRefundSuccess(rsp *alipay.TradeRefundRsp) error {
	if rsp == nil {
		return errors.New("alipay refund failed: empty response")
	}
	if rsp.Code.IsFailure() {
		return fmt.Errorf("alipay refund failed: %s %s", rsp.Code, rsp.SubMsg)
	}
	return nil
}

func (c *Client) trade(req PayRequest, productCode string) alipay.Trade {
	return alipay.Trade{
		NotifyURL:      firstNonEmpty(req.NotifyURL, c.cfg.NotifyURL),
		ReturnURL:      firstNonEmpty(req.ReturnURL, c.cfg.ReturnURL),
		Subject:        strings.TrimSpace(req.Subject),
		OutTradeNo:     strings.TrimSpace(req.OutTradeNo),
		TotalAmount:    strings.TrimSpace(req.TotalAmount),
		ProductCode:    productCode,
		Body:           strings.TrimSpace(req.Body),
		PassbackParams: strings.TrimSpace(req.PassbackParams),
		TimeoutExpress: strings.TrimSpace(req.TimeoutExpress),
	}
}

func normalizeConfig(cfg Config) Config {
	cfg.AppID = strings.TrimSpace(cfg.AppID)
	cfg.PrivateKey = strings.TrimSpace(cfg.PrivateKey)
	cfg.AlipayPublicKey = strings.TrimSpace(cfg.AlipayPublicKey)
	cfg.NotifyURL = strings.TrimSpace(cfg.NotifyURL)
	cfg.ReturnURL = strings.TrimSpace(cfg.ReturnURL)
	cfg.EncryptKey = strings.TrimSpace(cfg.EncryptKey)
	cfg.AppCertPublicKeyPath = strings.TrimSpace(cfg.AppCertPublicKeyPath)
	cfg.AlipayRootCertPath = strings.TrimSpace(cfg.AlipayRootCertPath)
	cfg.AlipayCertPublicKeyPath = strings.TrimSpace(cfg.AlipayCertPublicKeyPath)
	return cfg
}

func validatePayRequest(req PayRequest) error {
	if strings.TrimSpace(req.OutTradeNo) == "" {
		return errors.New("alipay out trade no is required")
	}
	if strings.TrimSpace(req.Subject) == "" {
		return errors.New("alipay subject is required")
	}
	if strings.TrimSpace(req.TotalAmount) == "" {
		return errors.New("alipay total amount is required")
	}
	return nil
}

func validateRefundRequest(req RefundRequest) error {
	if strings.TrimSpace(req.OutTradeNo) == "" && strings.TrimSpace(req.TradeNo) == "" {
		return errors.New("alipay out trade no or trade no is required")
	}
	if strings.TrimSpace(req.RefundAmount) == "" {
		return errors.New("alipay refund amount is required")
	}
	if strings.TrimSpace(req.OutRequestNo) == "" {
		return errors.New("alipay out request no is required")
	}
	return nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func hasAnyCertPath(cfg Config) bool {
	return cfg.AppCertPublicKeyPath != "" || cfg.AlipayRootCertPath != "" || cfg.AlipayCertPublicKeyPath != ""
}

func hasAllCertPaths(cfg Config) bool {
	return cfg.AppCertPublicKeyPath != "" && cfg.AlipayRootCertPath != "" && cfg.AlipayCertPublicKeyPath != ""
}
