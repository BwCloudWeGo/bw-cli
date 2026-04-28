package errors_test

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"

	apperrors "github.com/BwCloudWeGo/bw-cli/pkg/errors"
)

func TestAppErrorMapsToHTTPAndGRPCStatuses(t *testing.T) {
	cases := []struct {
		name       string
		err        error
		httpStatus int
		grpcCode   codes.Code
	}{
		{"invalid argument", apperrors.InvalidArgument("bad_request", "bad request"), http.StatusBadRequest, codes.InvalidArgument},
		{"unauthorized", apperrors.Unauthorized("auth_required", "auth required"), http.StatusUnauthorized, codes.Unauthenticated},
		{"not found", apperrors.NotFound("user_not_found", "user not found"), http.StatusNotFound, codes.NotFound},
		{"conflict", apperrors.Conflict("email_exists", "email exists"), http.StatusConflict, codes.AlreadyExists},
		{"internal", apperrors.Internal("internal_error", "internal error"), http.StatusInternalServerError, codes.Internal},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.httpStatus, apperrors.HTTPStatus(tc.err))
			require.Equal(t, tc.grpcCode, apperrors.GRPCCode(tc.err))
		})
	}
}

func TestFromGRPCStatusRoundTripsAppCode(t *testing.T) {
	err := apperrors.ToGRPC(apperrors.NotFound("note_not_found", "note not found"))

	appErr := apperrors.FromGRPC(err)

	require.Equal(t, "note_not_found", appErr.Code)
	require.Equal(t, apperrors.KindNotFound, appErr.Kind)
}
