// 第②批 A 段（目录实体）的**声明式档位台账**（ADR-0064 决策 8，形状沿用第①批
// disposition_face_ledger_test.go：登记档位 → 逐档验 → 反向锁「登记了却打不出」）。
//
// 为什么这一批能用一张表打完 10 个端点：目录实体共用 catalog engine
// （catalogUpdate / catalogDelete 各一份实现），所以「三义塌成一格」也只有一个成因。
// 改之前的形状是 spec 里带一个 `NotFoundMsg string` —— 引擎只能 errors.New 出匿名错误，
// 于是 ①「查不动」与「不存在」在 api 层分不出档，②引擎一处缺陷同时污染 5 类实体 × 2 操作。
// 现在 spec 带的是具名哨兵，10 个端点各挂一次 WithSentinel 即可分档。
//
// 三档都不需要播种（成功档由既有契约测试覆盖）：400 = id 非数字、404 = id 不存在、
// 500 = 删表 ⇒ 若「查不动」又被塌成 404，500 这档会立刻红。
package api

import (
	"fmt"
	"net/http"
	"strings"
	"testing"
)

// catalogFace 一类目录实体的三档声明。
type catalogFace struct {
	name   string // 端点名（进用例名）
	method string
	path   string // 含 %d 时用 missing id 填；非数字档直接字符串替换
	table  string // 删表注入用的表名
}

var catalogFaces = []catalogFace{
	{"PUT /admin/specialty/:id", http.MethodPut, "/api/admin/specialty/%d", "specialty"},
	{"DELETE /admin/specialty/:id", http.MethodDelete, "/api/admin/specialty/%d", "specialty"},
	{"PUT /admin/level/:id", http.MethodPut, "/api/admin/level/%d", "course_level"},
	{"DELETE /admin/level/:id", http.MethodDelete, "/api/admin/level/%d", "course_level"},
	{"PUT /admin/certificate-template/:id", http.MethodPut, "/api/admin/certificate-template/%d", "certificate_template"},
	{"DELETE /admin/certificate-template/:id", http.MethodDelete, "/api/admin/certificate-template/%d", "certificate_template"},
	{"PUT /admin/question-tag/:id", http.MethodPut, "/api/admin/question-tag/%d", "question_tag"},
	{"DELETE /admin/question-tag/:id", http.MethodDelete, "/api/admin/question-tag/%d", "question_tag"},
	{"PUT /admin/credential/:id", http.MethodPut, "/api/admin/credential/%d", "credential"},
	{"DELETE /admin/credential/:id", http.MethodDelete, "/api/admin/credential/%d", "credential"},
}

const catalogMissingID = 999999

// TestCatalogFaceLedger 每个目录端点三档：输入不合法(400) / 真不存在(404) / 查不动(500)。
func TestCatalogFaceLedger(t *testing.T) {
	for _, f := range catalogFaces {
		for _, want := range []int{http.StatusBadRequest, http.StatusNotFound, http.StatusInternalServerError} {
			t.Run(fmt.Sprintf("%s/%d", f.name, want), func(t *testing.T) {
				r, db, token := newAdminContractEnv(t)

				var path string
				switch want {
				case http.StatusBadRequest:
					// 路径参数非数字：归位 400（票8 翻转后 *ParseError 恒优先，这里钉它没被吞回 404）。
					path = strings.Replace(fmt.Sprintf(f.path, catalogMissingID),
						fmt.Sprint(catalogMissingID), "not-a-number", 1)
				case http.StatusNotFound:
					path = fmt.Sprintf(f.path, catalogMissingID)
				case http.StatusInternalServerError:
					path = fmt.Sprintf(f.path, 1)
					if err := db.Exec("DROP TABLE " + f.table).Error; err != nil {
						t.Fatalf("注入故障（删 %s 表）失败: %v", f.table, err)
					}
				}

				body := map[string]any{"code": "LEDGER", "name": "台账实体"}
				var req any
				if f.method != http.MethodDelete {
					req = body
				}
				rec := doWithToken(t, r, token, f.method, path, req)
				if rec.Code != want {
					t.Fatalf("%s：声明档位 %d 打不出来，实得 %d，body=%s", f.name, want, rec.Code, rec.Body.String())
				}
				if out := rec.Body.String(); strings.Contains(out, "record not found") || strings.Contains(out, "no such table") {
					t.Fatalf("%s 把驱动原文吐进响应体: %s", f.name, out)
				}
			})
		}
	}
}
