package alipayx

import (
	"context"
	"testing"

	alipay "github.com/smartwalle/alipay/v3"
	"github.com/stretchr/testify/require"
)

func TestPagePayURLRejectsNilClient(t *testing.T) {
	var client *Client

	payURL, err := client.PagePayURL(PayRequest{
		OutTradeNo:  "order-1",
		Subject:     "order",
		TotalAmount: "9.90",
	})

	require.Empty(t, payURL)
	require.ErrorContains(t, err, "alipay client is nil")
}

func TestRefundOKRejectsNilClient(t *testing.T) {
	var client *Client

	err := client.RefundOK(context.Background(), RefundRequest{
		OutTradeNo:   "order-1",
		RefundAmount: "9.90",
		OutRequestNo: "refund-1",
	})

	require.ErrorContains(t, err, "alipay client is nil")
}

func TestEnsureRefundSuccessAcceptsSuccessCode(t *testing.T) {
	err := ensureRefundSuccess(&alipay.TradeRefundRsp{
		Error: alipay.Error{Code: alipay.CodeSuccess},
	})

	require.NoError(t, err)
}

func TestEnsureRefundSuccessReturnsReadableFailure(t *testing.T) {
	err := ensureRefundSuccess(&alipay.TradeRefundRsp{
		Error: alipay.Error{
			Code:   alipay.CodeInvalidParam,
			SubMsg: "invalid refund amount",
		},
	})

	require.ErrorContains(t, err, "alipay refund failed")
	require.ErrorContains(t, err, "40002")
	require.ErrorContains(t, err, "invalid refund amount")
}
