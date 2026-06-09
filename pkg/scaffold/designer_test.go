package scaffold

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDesignerPlanEndpointSavesPlan(t *testing.T) {
	root := t.TempDir()
	mux := newDesignerMux(DesignerOptions{RootDir: root})
	body := strings.NewReader(`{"service_name":"product","root_table":"product_spu","tables":["product_spu"],"relationships":[]}`)
	req := httptest.NewRequest(http.MethodPost, "/api/plan", body)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.FileExists(t, filepath.Join(root, "scaffold-plans", "product.json"))
}
