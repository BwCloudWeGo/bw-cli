package alipayx_test

import (
	"context"
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/BwCloudWeGo/bw-cli/pkg/alipayx"
)

const testPrivateKey = `-----BEGIN PRIVATE KEY-----
MIICdQIBADANBgkqhkiG9w0BAQEFAASCAl8wggJbAgEAAoGBAMiVMWPJXP2B9fWK
v18CENwPZbJfOasLI9MHurbzs6rCvlXTmiG+hsWcTw8KkoefFK3MfqUBIxnOM5yb
ai6l7f1ODfF01qzk0FudSbkSQsA87tXZdLbwxEEbbWBNbs4BPBirLvxa7AsVAto0
iHEcq2IKLt87h4MYGfP4vZwuSBXNAgMBAAECgYBxVYswnMhEHTiCYsE6x4oLLVAC
9zc4Y/T7+jQPx6dO5vZwvD0sr+Cqq2UoVIrywnoGsbMlPH0+yXn0FQRsEylio6a9
vKdSybLa6fW26sWEua+ZlIHemGFvHQ9XNrlJbSKgM4T9HvC5bs9L6KXsSLNQUcqI
P1Y91PkG+2IkihKiwQJBAPgDqndHjDjdkHDEl59dBnMEF8hUEO0ziu3OjwnPlzv1
j6wNPDmdXq9U7J20FtujdYHBEh2sP5f9nuLH4tRiJqUCQQDPCo7djpQTUZk9hb8t
+4GTPoXnA/NbvMte+nHRiPO47ZC9D1tOjJKZlFuRCY0Meo2wPmO0DUCva2EnUX3s
PrIJAkA0y5r7H0jzRf8cck0QiJ351/I0G+kqhWFatDDw1rcL9X8rEfozDZP9YOep
vo9rHAXEpFP16xfyg/PRtNlNesNdAkAjGr8ugcZJoERDUjIgMcy+kpNRoDHbFB/H
ct9pj7cDXAR2iewJXXxd3fHInb30p7LudyWgmb6l/6bxa7fWHqtBAkAbJuWa15Rn
NgXtScAjuPVNbTIztwpCG+sBp3zZPVitEqrmiUfpcAP7XuuMHhTHn2WVJ0+icZY1
jN0pFHTUdTCS
-----END PRIVATE KEY-----`

func TestNewClientRequiresAppIDAndPrivateKey(t *testing.T) {
	client, err := alipayx.NewClient(alipayx.Config{AppID: "app-id"})

	require.Nil(t, client)
	require.EqualError(t, err, "alipay private key is required")
}

func TestPagePayUsesDefaultURLsAndProductCode(t *testing.T) {
	client, err := alipayx.NewClient(alipayx.Config{
		AppID:      "2021000000000000",
		PrivateKey: testPrivateKey,
		NotifyURL:  "https://api.example.com/payments/alipay/notify",
		ReturnURL:  "https://www.example.com/orders/paid",
	})
	require.NoError(t, err)

	payURL, err := client.PagePay(alipayx.PayRequest{
		OutTradeNo:  "order-1001",
		Subject:     "小蓝书会员",
		TotalAmount: "19.90",
	})

	require.NoError(t, err)
	values := payURL.Query()
	require.Equal(t, "alipay.trade.page.pay", values.Get("method"))
	require.Equal(t, "https://api.example.com/payments/alipay/notify", values.Get("notify_url"))
	require.Equal(t, "https://www.example.com/orders/paid", values.Get("return_url"))
	require.Contains(t, values.Get("biz_content"), `"out_trade_no":"order-1001"`)
	require.Contains(t, values.Get("biz_content"), `"product_code":"FAST_INSTANT_TRADE_PAY"`)
}

func TestAppPayReturnsEncodedOrderString(t *testing.T) {
	client, err := alipayx.NewClient(alipayx.Config{
		AppID:      "2021000000000000",
		PrivateKey: testPrivateKey,
		NotifyURL:  "https://api.example.com/payments/alipay/notify",
	})
	require.NoError(t, err)

	orderString, err := client.AppPay(alipayx.PayRequest{
		OutTradeNo:  "order-1002",
		Subject:     "小蓝书课程",
		TotalAmount: "99.00",
	})

	require.NoError(t, err)
	values, err := url.ParseQuery(orderString)
	require.NoError(t, err)
	require.Equal(t, "alipay.trade.app.pay", values.Get("method"))
	require.Equal(t, "https://api.example.com/payments/alipay/notify", values.Get("notify_url"))
	require.Contains(t, values.Get("biz_content"), `"product_code":"QUICK_MSECURITY_PAY"`)
}

func TestDecodeNotificationVerifiesSignature(t *testing.T) {
	client, err := alipayx.NewClient(alipayx.Config{
		AppID:      "2021000000000000",
		PrivateKey: testPrivateKey,
		AlipayPublicKey: `-----BEGIN PUBLIC KEY-----
MIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEAw9GqT0tD0jYcOgcID1yR
2B8HHJXXzzyGkWbWHZ2rWCW4BfW02mo+axh3eIt//R5CeuCv9r1GONpdyW4nCixn
PNM3ru9u4gI4BeEzxS/ZSIYxD1tVUiDtnCoUKVKkUD6qUpK6oTL2iZuSJAJoPCbO
MwK+TDKf4RHqxHoUQyxJHoTVz/iA3/QK2bnHNeFXrC+ZkNdHVjgxp7DGgRwvr34f
w0x1fE+IinVH+FRjB7W1v/kUxM0UlLDNrmntTih7ktF6wYc7OHuydnn1i4hv1HNn
10jS/ZoSxclguLzS+QJxYaKSkR7BxsaXxwEEgvbfYHJr2JXnkk0zz9dJ9vqTg3RC
BwIDAQAB
-----END PUBLIC KEY-----`,
	})
	require.NoError(t, err)

	notification, err := client.DecodeNotification(context.Background(), url.Values{
		"app_id":       {"2021000000000000"},
		"out_trade_no": {"order-1001"},
		"trade_status": {"TRADE_SUCCESS"},
		"sign":         {"bad-sign"},
		"sign_type":    {"RSA2"},
	})

	require.Nil(t, notification)
	require.Error(t, err)
}
