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

普通公钥模式和证书模式二选一。不要同时配置 `alipay_public_key` 和证书路径。

## 初始化

在进程入口初始化一次，然后注入到业务 service：

```go
if err := config.InitGlobal("configs/config.yaml"); err != nil {
    panic(err)
}
cfg := config.MustGlobal()

payClient, err := alipayx.NewClient(cfg.Alipay)
if err != nil {
    panic(err)
}

orderSvc := orderservice.NewService(repo, payClient, log)
```

调用关系保持简单：

```text
handler -> order service -> alipayx.Client
```

## 创建支付

实际业务 service 中直接调用 `pkg/alipayx`，不要在业务项目里再复制一层支付宝工具类。

```go
func (s *Service) CreateAlipayPagePayment(ctx context.Context, orderID string) (string, error) {
    order, err := s.repo.FindByID(ctx, orderID)
    if err != nil {
        return "", err
    }
    if order.PaidAt != nil {
        return "", apperrors.FailedPrecondition("ORDER_ALREADY_PAID", "order already paid")
    }

    return s.alipay.PagePayURL(alipayx.PayRequest{
        OutTradeNo:     order.PayNo,
        Subject:        "小蓝书订单",
        TotalAmount:    order.PayAmount.StringFixed(2),
        Body:           "订单支付",
        TimeoutExpress: "15m",
    })
}

func (s *Service) CreateAlipayWapPayment(ctx context.Context, orderID string) (string, error) {
    order, err := s.repo.FindByID(ctx, orderID)
    if err != nil {
        return "", err
    }

    return s.alipay.WapPayURL(alipayx.PayRequest{
        OutTradeNo:  order.PayNo,
        Subject:     "小蓝书订单",
        TotalAmount: order.PayAmount.StringFixed(2),
    })
}

func (s *Service) CreateAlipayAppPayment(ctx context.Context, orderID string) (string, error) {
    order, err := s.repo.FindByID(ctx, orderID)
    if err != nil {
        return "", err
    }

    return s.alipay.AppPay(alipayx.PayRequest{
        OutTradeNo:  order.PayNo,
        Subject:     "小蓝书订单",
        TotalAmount: order.PayAmount.StringFixed(2),
    })
}
```

Gin handler 只处理协议入参和出参：

```go
func CreateAlipayPagePayment(svc *orderservice.Service) gin.HandlerFunc {
    return func(c *gin.Context) {
        var req struct {
            OrderID string `json:"order_id" binding:"required"`
        }
        if err := c.ShouldBindJSON(&req); err != nil {
            httpx.Error(c, apperrors.InvalidArgument("INVALID_REQUEST", err.Error()))
            return
        }

        payURL, err := svc.CreateAlipayPagePayment(c.Request.Context(), req.OrderID)
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
func AlipayNotify(payClient *alipayx.Client, svc *orderservice.Service) gin.HandlerFunc {
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

        err = svc.MarkAlipayPaid(
            c.Request.Context(),
            notification.OutTradeNo,
            notification.TradeNo,
            notification.TotalAmount,
            notification.TradeStatus,
        )
        if err != nil {
            c.String(http.StatusInternalServerError, "fail")
            return
        }

        c.String(http.StatusOK, "success")
    }
}
```

`MarkAlipayPaid` 建议至少校验：

1. `OutTradeNo` 属于本系统订单。
2. `TotalAmount` 等于订单应付金额。
3. `TradeStatus` 为 `TRADE_SUCCESS` 或 `TRADE_FINISHED`。
4. 通知处理幂等，重复通知直接返回成功。

## 退款

业务 service 只关心退款是否成功，支付宝响应码判断由 `pkg/alipayx` 封装。

```go
func (s *Service) RefundAlipayOrder(ctx context.Context, orderID string, reason string) error {
    order, err := s.repo.FindByID(ctx, orderID)
    if err != nil {
        return err
    }
    if order.RefundedAt != nil {
        return nil
    }

    return s.alipay.RefundOK(ctx, alipayx.RefundRequest{
        OutTradeNo:   order.PayNo,
        RefundAmount: order.PayAmount.StringFixed(2),
        RefundReason: reason,
        OutRequestNo: order.PayNo + "-refund-001",
    })
}
```

多次部分退款时，`OutRequestNo` 必须为每次退款请求生成唯一值。
