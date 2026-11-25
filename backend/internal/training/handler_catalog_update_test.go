package training

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"forklift-training/internal/testutil"
	"forklift-training/pkg/response"
)

func TestCatalogUpdateEndpointsParseErrorPriority(t *testing.T) {
	testutil.SetTestGinMode()
	h := newHandler(nil)
	endpoints := []struct {
		name   string
		path   string
		param  string
		idMsg  string
		handle gin.HandlerFunc
	}{
		{"specialty", "/specialty/:specialty_id", "specialty_id", "专业方向ID无效", h.UpdateSpecialty},
		{"level", "/level/:level_id", "level_id", "课程等级ID无效", h.UpdateLevel},
		{"certificate-template", "/certificate-template/:id", "id", "证书模板ID无效", h.UpdateCertificateTemplate},
		{"question-tag", "/question-tag/:id", "id", "题库标签ID无效", h.UpdateQuestionTag},
		{"credential", "/credential/:id", "id", "证件ID无效", h.UpdateCredential},
	}

	for _, endpoint := range endpoints {
		t.Run(endpoint.name, func(t *testing.T) {
			cases := []struct {
				name    string
				id      string
				wantMsg string
			}{
				{"path_error_precedes_body_error", "not-a-number", endpoint.idMsg},
				{"body_error", "1", "请求数据无效"},
			}
			for _, tc := range cases {
				t.Run(tc.name, func(t *testing.T) {
					router := gin.New()
					router.PUT(endpoint.path, endpoint.handle)
					path := strings.Replace(endpoint.path, ":"+endpoint.param, tc.id, 1)
					req := httptest.NewRequest(http.MethodPut, path, strings.NewReader("{"))
					req.Header.Set("Content-Type", "application/json")
					recorder := httptest.NewRecorder()

					router.ServeHTTP(recorder, req)

					var out response.R
					if err := json.NewDecoder(recorder.Body).Decode(&out); err != nil {
						t.Fatalf("decode response: %v; body=%s", err, recorder.Body.String())
					}
					if recorder.Code != http.StatusBadRequest || out.Code != http.StatusBadRequest {
						t.Fatalf("status = HTTP %d, envelope %d; want 400", recorder.Code, out.Code)
					}
					if out.Message != tc.wantMsg {
						t.Fatalf("message = %q; want %q", out.Message, tc.wantMsg)
					}
				})
			}
		})
	}
}
