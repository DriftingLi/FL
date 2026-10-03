// 本文件把 fetchRecruiters 的**一份**留在 package api。
//
// #1445 批 4 把 recruiter_list_contract_test.go 沉到 internal/admin，但留在 api 的
// recruiter_edit_contract_test.go:78 仍要用它读列表（那个文件走全量装配链，本轮不动），
// 故按 `forum_topic_list_resp_test.go` 的先例在 api 侧留一份；正文逐字来自原文件。
package api

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
)

func fetchRecruiters(t *testing.T, r *gin.Engine, token, query string) ([]map[string]any, float64) {
	t.Helper()
	path := `/api/admin/recruiters`
	if query != `` {
		path += `?` + query
	}
	rec := doWithToken(t, r, token, http.MethodGet, path, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf(`GET %s should be 200, got %d body=%s`, path, rec.Code, rec.Body.String())
	}
	var env map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatalf(`parse envelope failed: %v`, err)
	}
	data, _ := env[`data`].(map[string]any)
	items := []map[string]any{}
	if raw, ok := data[`items`].([]any); ok {
		for _, it := range raw {
			if m, ok := it.(map[string]any); ok {
				items = append(items, m)
			}
		}
	}
	total, _ := data[`total`].(float64)
	return items, total
}
