# 支付宝支付调用示例

本文档说明如何使用 `pkg/alipayx` 接入支付宝支付、同步回调验签、异步通知和退款。底层 SDK 使用 `github.com/smartwalle/alipay/v3`。

## 配置

普通公钥模式：

```yaml
alipay:
  app_id: ""
  private_key: ""
  alipay_public_key: ""
  production: false
  notify_url: "https://api.example.com/payments/alipay/notify"
  return_url: "https://www.example.com/orders/alipay/return"
  encrypt_key: ""
```

证书模式：

```yaml
alipay:
  app_id: ""
  private_key: ""
  production: true
  notify_url: "https://api.example.com/payments/alipay/notify"
  return_url: "https://www.example.com/orders/alipay/return"
  app_cert_public_key_path: "configs/certs/appCertPublicKey.crt"
  alipay_root_cert_path: "configs/certs/alipayRootCert.crt"
  alipay_cert_public_key_path: "configs/certs/alipayCertPublicKey_RSA2.crt"
```

常用环境变量：

```bash
export APP_ALIPAY_APP_ID='2021000000000000'
export APP_ALIPAY_PRIVATE_KEY='-----BEGIN PRIVATE KEY-----...'
export APP_ALIPAY_ALIPAY_PUBLIC_KEY='-----BEGIN PUBLIC KEY-----...'
export APP_ALIPAY_PRODUCTION=false
export APP_ALIPAY_NOTIFY_URL='https://api.example.com/payments/alipay/notify'
export APP_ALIPAY_RETURN_URL='https://www.example.com/orders/alipay/return'
```

普通公钥模式和证书模式二选一。不要同时配置 `alipay_public_key` 和证书路径。

## 初始化

```go
if err := config.InitGlobal("configs/config.yaml"); err != nil {
    panic(err)
}
cfg := config.MustGlobal()

payClient, err := alipayx.NewClient(cfg.Alipay)
if err != nil {
    panic(err)
}
```

推荐在入口初始化一次，然后注入业务 service：

```text
handler -> service -> alipayx.Client
```

## 创建支付

```go
type PaymentService struct {
    alipay *alipayx.Client
}

func NewPaymentService(alipay *alipayx.Client) *PaymentService {
    return &PaymentService{alipay: alipay}
}

func (s *PaymentService) CreatePagePayment(ctx context.Context, orderID string, amount string) (string, error) {
    payURL, err := s.alipay.PagePay(alipayx.PayRequest{
        OutTradeNo:     orderID,
        Subject:        "小蓝书订单",
        TotalAmount:    amount,
        Body:           "订单支付",
        TimeoutExpress: "15m",
    })
    if err != nil {
        return "", err
    }
    return payURL.String(), nil
}

func (s *PaymentService) CreateWapPayment(ctx context.Context, orderID string, amount string) (string, error) {
    payURL, err := s.alipay.WapPay(alipayx.PayRequest{
        OutTradeNo:  orderID,
        Subject:     "小蓝书订单",
        TotalAmount: amount,
    })
    if err != nil {
        return "", err
    }
    return payURL.String(), nil
}

func (s *PaymentService) CreateAppPayment(ctx context.Context, orderID string, amount string) (string, error) {
    return s.alipay.AppPay(alipayx.PayRequest{
        OutTradeNo:  orderID,
        Subject:     "小蓝书订单",
        TotalAmount: amount,
    })
}
```

Gin handler 示例：

```go
func CreateAlipayPagePayment(svc *PaymentService) gin.HandlerFunc {
    return func(c *gin.Context) {
        var req struct {
            OrderID string `json:"order_id" binding:"required"`
            Amount  string `json:"amount" binding:"required"`
        }
        if err := c.ShouldBindJSON(&req); err != nil {
            httpx.Error(c, apperrors.InvalidArgument("INVALID_REQUEST", err.Error()))
            return
        }

        payURL, err := svc.CreatePagePayment(c.Request.Context(), req.OrderID, req.Amount)
        if err != nil {
            httpx.Error(c, err)
            return
        }
        httpx.OK(c, gin.H{"pay_url": payURL})
    }
}
```

## 同步回调验签

同步回调来自 `return_url`，适合展示支付结果页。最终到账状态应以异步通知或主动查询为准。

```go
func AlipayReturn(payClient *alipayx.Client) gin.HandlerFunc {
    return func(c *gin.Context) {
        if err := c.Request.ParseForm(); err != nil {
            httpx.Error(c, apperrors.InvalidArgument("INVALID_REQUEST", err.Error()))
            return
        }
        if err := payClient.VerifyReturn(c.Request.Context(), c.Request.Form); err != nil {
            httpx.Error(c, apperrors.InvalidArgument("ALIPAY_SIGN_INVALID", err.Error()))
            return
        }

        httpx.OK(c, gin.H{
            "out_trade_no": c.Request.Form.Get("out_trade_no"),
            "trade_no":     c.Request.Form.Get("trade_no"),
        })
    }
}
```

## 异步通知回调

支付宝异步通知必须验签、校验订单号和金额、幂等更新订单状态。处理成功后返回纯文本 `success`。

```go
func AlipayNotify(payClient *alipayx.Client, svc *PaymentService) gin.HandlerFunc {
    return func(c *gin.Context) {
        if err := c.Request.ParseForm(); err != nil {
            c.String(http.StatusBadRequest, "fail")
            return
        }

        notification, err := payClient.DecodeNotification(c.Request.Context(), c.Request.Form)
        if err != nil {
            c.String(http.StatusBadRequest, "fail")
            return
        }

        err = svc.MarkPaid(c.Request.Context(), notification.OutTradeNo, notification.TradeNo, notification.TotalAmount)
        if err != nil {
            c.String(http.StatusInternalServerError, "fail")
            return
        }

        c.String(http.StatusOK, "success")
    }
}
```

`MarkPaid` 建议至少校验：

1. `OutTradeNo` 属于本系统订单。
2. `TotalAmount` 等于订单应付金额。
3. `TradeStatus` 为 `TRADE_SUCCESS` 或 `TRADE_FINISHED`。
4. 通知处理幂等，重复通知直接返回成功。

## 退款

```go
func (s *PaymentService) Refund(ctx context.Context, orderID string, amount string, reason string) error {
    rsp, err := s.alipay.Refund(ctx, alipayx.RefundRequest{
        OutTradeNo:   orderID,
        RefundAmount: amount,
        RefundReason: reason,
        OutRequestNo: orderID + "-refund-001",
    })
    if err != nil {
        return err
    }
    if rsp.Code.IsFailure() {
        return fmt.Errorf("alipay refund failed: %s %s", rsp.Code, rsp.SubMsg)
    }
    return nil
}
```

多次部分退款时，`OutRequestNo` 必须为每次退款请求生成唯一值。
