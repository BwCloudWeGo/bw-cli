// Package alipayx wraps github.com/smartwalle/alipay/v3 with framework-friendly
// configuration and narrow payment, notification and refund helpers.
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
	// ProductCodePagePay is the product code used by Alipay PC website payment.
	ProductCodePagePay = "FAST_INSTANT_TRADE_PAY"
	// ProductCodeWapPay is the product code used by Alipay mobile website payment.
	ProductCodeWapPay = "QUICK_WAP_WAY"
	// ProductCodeAppPay is the product code used by Alipay app payment.
	ProductCodeAppPay = "QUICK_MSECURITY_PAY"
)

// Config contains Alipay application credentials and callback defaults.
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

// DefaultConfig returns local-development Alipay defaults.
func DefaultConfig() Config {
	return Config{}
}

// Client provides the framework's narrow Alipay payment surface.
type Client struct {
	cfg Config
	raw *alipay.Client
}

// PayRequest describes one Alipay payment order.
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

// RefundRequest describes one Alipay refund request.
type RefundRequest struct {
	OutTradeNo   string
	TradeNo      string
	RefundAmount string
	RefundReason string
	OutRequestNo string
}

// NewClient creates a configured smartwalle Alipay client.
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

// Raw returns the underlying smartwalle client for uncommon Alipay APIs.
func (c *Client) Raw() *alipay.Client {
	if c == nil {
		return nil
	}
	return c.raw
}

// PagePay builds the PC website payment redirect URL.
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

// PagePayURL builds the PC website payment redirect URL as a string.
func (c *Client) PagePayURL(req PayRequest) (string, error) {
	payURL, err := c.PagePay(req)
	if err != nil {
		return "", err
	}
	return payURL.String(), nil
}

// WapPay builds the mobile website payment redirect URL.
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

// WapPayURL builds the mobile website payment redirect URL as a string.
func (c *Client) WapPayURL(req PayRequest) (string, error) {
	payURL, err := c.WapPay(req)
	if err != nil {
		return "", err
	}
	return payURL.String(), nil
}

// AppPay builds the order string that mobile clients pass to the Alipay SDK.
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

// DecodeNotification verifies an async notification and converts it into a typed payload.
func (c *Client) DecodeNotification(ctx context.Context, values url.Values) (*alipay.Notification, error) {
	if c == nil || c.raw == nil {
		return nil, errors.New("alipay client is nil")
	}
	return c.raw.DecodeNotification(ctx, values)
}

// VerifyReturn verifies signed query/form values from a synchronous return_url redirect.
func (c *Client) VerifyReturn(ctx context.Context, values url.Values) error {
	if c == nil || c.raw == nil {
		return errors.New("alipay client is nil")
	}
	return c.raw.VerifySign(ctx, values)
}

// Refund submits a synchronous refund request to Alipay.
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

// RefundOK submits a refund request and returns an error when Alipay rejects it.
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
