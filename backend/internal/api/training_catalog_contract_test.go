// Tickets #85/#86/#87 契约测试：专业方向/课程等级/证书模板/题库标签 CRUD 端点
// typed 化后，响应 JSON 必须与旧 map 字典字节级一致（字段键集与键序不变）。
// 键序由 service 层 DTO 序列化测试锁定（Test*DictJSON），本测试锁定端点信封、
// 状态码与 data 键集/值类型（map 反序列化后全部数值为 float64）。
package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"forklift-training/internal/config"
	"forklift-training/internal/security"
	"forklift-training/internal/testutil"
)

func newCatalogContractEnv(t *testing.T) (*gin.Engine, *config.Config, *Deps) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	cfg := &config.Config{
		JWTSecretKey: "contract-test-secret",
		AuthCookie:   config.AuthCookieConfig{Name: "hrwai_token"},
	}
	db := testutil.NewMemoryDB(t)
	testutil.SeedAdmin(t, db, "admin1", "hash123")
	deps := newContractDeps(t, db, cfg)
	return NewRouter(deps), cfg, deps
}

func catalogAdminToken(t *testing.T, cfg *config.Config, adminID int) string {
	t.Helper()
	token, err := security.NewSession(cfg.JWTSecretKey, time.Hour, security.CookieConfig{}).
		Issue(adminID, "admin1", "admin")
	if err != nil {
		t.Fatalf("签发 token 失败: %v", err)
	}
	return token
}

func catalogRequest(t *testing.T, r *gin.Engine, token, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	var rd io.Reader
	if body != "" {
		rd = strings.NewReader(body)
	}
	req, _ := http.NewRequest(method, path, rd)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// unpackData 解析信封 {code, message, data}，返回 data 原文。
func unpackData(t *testing.T, rec *httptest.ResponseRecorder, wantCode int) (int, string, string) {
	t.Helper()
	if rec.Code != wantCode {
		t.Fatalf("期望 HTTP %d, got %d: %s", wantCode, rec.Code, rec.Body.String())
	}
	var body struct {
		Code    int             `json:"code"`
		Message string          `json:"message"`
		Data    json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("解析响应失败: %v", err)
	}
	if body.Code != wantCode {
		t.Fatalf("期望 code=%d, got %d: %s", wantCode, body.Code, rec.Body.String())
	}
	return body.Code, body.Message, string(body.Data)
}

// assertDictKeys 断言字典对象键集与旧 map 字典完全一致（map 序列化按键排序，
// 反序列化后键集一致 + service 层键序测试 → 字节级契约锁定）。
func assertDictKeys(t *testing.T, got map[string]any, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("字典键数不符: got %d keys %v, want %d", len(got), keysOf(got), len(want))
	}
	for _, k := range want {
		if _, ok := got[k]; !ok {
			t.Fatalf("缺少字段 %q: %v", k, keysOf(got))
		}
	}
}

func keysOf(m map[string]any) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

var isoMicroRE = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}\.\d{6}$`)

// TestCatalogContract_Specialty 专业方向端点全流程：创建/列表/更新/交换/删除，
// data 键集与旧 map 字典一致，值类型经 JSON 往返后不变。
func TestCatalogContract_Specialty(t *testing.T) {
	r, cfg, _ := newCatalogContractEnv(t)
	token := catalogAdminToken(t, cfg, 1)

	// 创建
	rec := catalogRequest(t, r, token, "POST", "/api/admin/specialty",
		`{"code":"operation","name":"操作","sort_order":1}`)
	_, _, data := unpackData(t, rec, http.StatusCreated)
	var created map[string]any
	if err := json.Unmarshal([]byte(data), &created); err != nil {
		t.Fatalf("解析创建响应失败: %v", err)
	}
	assertDictKeys(t, created, []string{"code", "created_at", "description", "name", "sort_order", "specialty_id", "status"})
	if created["code"] != "operation" || created["name"] != "操作" ||
		created["sort_order"] != float64(1) || created["status"] != float64(1) ||
		created["description"] != "" {
		t.Fatalf("创建返回字段不匹配: %+v", created)
	}
	createdAt, _ := created["created_at"].(string)
	if !isoMicroRE.MatchString(createdAt) {
		t.Fatalf("created_at 格式异常: %q", createdAt)
	}
	specID := int(created["specialty_id"].(float64))

	// 编码必填 / 重复编码 / 非法状态（校验收口在 service）
	rec = catalogRequest(t, r, token, "POST", "/api/admin/specialty", `{"name":"X"}`)
	_, msg, _ := unpackData(t, rec, http.StatusBadRequest)
	if msg != "专业方向编码不能为空" {
		t.Fatalf("编码必填消息不符: %q", msg)
	}
	rec = catalogRequest(t, r, token, "POST", "/api/admin/specialty", `{"code":"operation","name":"重复"}`)
	_, msg, _ = unpackData(t, rec, http.StatusBadRequest)
	if msg != "专业方向编码已存在" {
		t.Fatalf("重复编码消息不符: %q", msg)
	}
	rec = catalogRequest(t, r, token, "POST", "/api/admin/specialty", `{"code":"x","name":"X","status":5}`)
	_, msg, _ = unpackData(t, rec, http.StatusBadRequest)
	if msg != "状态值无效" {
		t.Fatalf("非法状态消息不符: %q", msg)
	}

	// 列表（管理端含停用项）
	rec = catalogRequest(t, r, token, "GET", "/api/admin/specialties", "")
	_, _, data = unpackData(t, rec, http.StatusOK)
	var list struct {
		Specialties []map[string]any `json:"specialties"`
	}
	if err := json.Unmarshal([]byte(data), &list); err != nil || len(list.Specialties) != 1 {
		t.Fatalf("列表解析失败: %v, data=%s", err, data)
	}
	assertDictKeys(t, list.Specialties[0], []string{"code", "created_at", "description", "name", "sort_order", "specialty_id", "status"})

	// 学员端公开列表（仅启用项）：当前启用，1 条
	rec = catalogRequest(t, r, "", "GET", "/api/specialties", "")
	_, _, data = unpackData(t, rec, http.StatusOK)
	if err := json.Unmarshal([]byte(data), &list); err != nil || len(list.Specialties) != 1 {
		t.Fatalf("学员端列表解析失败: %v, data=%s", err, data)
	}

	// 更新（改名 + 停用）
	rec = catalogRequest(t, r, token, "PUT", fmt.Sprintf("/api/admin/specialty/%d", specID),
		`{"name":"操作方向","status":0}`)
	_, _, data = unpackData(t, rec, http.StatusOK)
	var updated map[string]any
	if err := json.Unmarshal([]byte(data), &updated); err != nil {
		t.Fatalf("解析更新响应失败: %v", err)
	}
	assertDictKeys(t, updated, []string{"code", "created_at", "description", "name", "sort_order", "specialty_id", "status"})
	if updated["name"] != "操作方向" || updated["status"] != float64(0) {
		t.Fatalf("更新返回字段不匹配: %+v", updated)
	}

	// 停用后学员端不可见、管理端仍可见
	rec = catalogRequest(t, r, "", "GET", "/api/specialties", "")
	_, _, data = unpackData(t, rec, http.StatusOK)
	if err := json.Unmarshal([]byte(data), &list); err != nil || len(list.Specialties) != 0 {
		t.Fatalf("停用项不应出现在学员端列表: %s", data)
	}

	// 交换排序：再建一个，交换后顺序翻转
	rec = catalogRequest(t, r, token, "POST", "/api/admin/specialty", `{"code":"maintenance","name":"维修"}`)
	_, _, data = unpackData(t, rec, http.StatusCreated)
	var second map[string]any
	if err := json.Unmarshal([]byte(data), &second); err != nil {
		t.Fatalf("解析创建响应失败: %v", err)
	}
	secondID := int(second["specialty_id"].(float64))
	rec = catalogRequest(t, r, token, "PUT",
		fmt.Sprintf("/api/admin/specialty/%d/sort", secondID), `{"swap_with":`+fmt.Sprint(specID)+`}`)
	_, _, _ = unpackData(t, rec, http.StatusOK)
	rec = catalogRequest(t, r, token, "GET", "/api/admin/specialties", "")
	_, _, data = unpackData(t, rec, http.StatusOK)
	if err := json.Unmarshal([]byte(data), &list); err != nil {
		t.Fatalf("列表解析失败: %v", err)
	}
	if list.Specialties[0]["name"] != "维修" || list.Specialties[1]["name"] != "操作方向" {
		t.Fatalf("交换后顺序应为 维修,操作方向: %+v", list.Specialties)
	}

	// 删除 + 不存在
	rec = catalogRequest(t, r, token, "DELETE", fmt.Sprintf("/api/admin/specialty/%d", specID), "")
	unpackData(t, rec, http.StatusOK)
	rec = catalogRequest(t, r, token, "DELETE", fmt.Sprintf("/api/admin/specialty/%d", specID), "")
	_, msg, _ = unpackData(t, rec, http.StatusNotFound)
	if msg != "专业方向不存在" {
		t.Fatalf("删除不存在消息不符: %q", msg)
	}

	// 更新不存在的方向
	rec = catalogRequest(t, r, token, "PUT", "/api/admin/specialty/99999", `{"name":"X"}`)
	_, msg, _ = unpackData(t, rec, http.StatusNotFound)
	if msg != "专业方向不存在" {
		t.Fatalf("更新不存在消息不符: %q", msg)
	}
}

// TestCatalogContract_Level 课程等级端点：创建/列表/更新/删除键集一致。
func TestCatalogContract_Level(t *testing.T) {
	r, cfg, _ := newCatalogContractEnv(t)
	token := catalogAdminToken(t, cfg, 1)

	rec := catalogRequest(t, r, token, "POST", "/api/admin/level", `{"code":"beginner","name":"入门"}`)
	_, _, data := unpackData(t, rec, http.StatusCreated)
	var created map[string]any
	if err := json.Unmarshal([]byte(data), &created); err != nil {
		t.Fatalf("解析创建响应失败: %v", err)
	}
	assertDictKeys(t, created, []string{"code", "created_at", "description", "level_id", "name", "sort_order", "status"})
	if created["code"] != "beginner" || created["name"] != "入门" || created["status"] != float64(1) {
		t.Fatalf("创建返回字段不匹配: %+v", created)
	}
	levelID := int(created["level_id"].(float64))

	rec = catalogRequest(t, r, "", "GET", "/api/levels", "")
	_, _, data = unpackData(t, rec, http.StatusOK)
	var list struct {
		Levels []map[string]any `json:"levels"`
	}
	if err := json.Unmarshal([]byte(data), &list); err != nil || len(list.Levels) != 1 {
		t.Fatalf("学员端等级列表解析失败: %v, data=%s", err, data)
	}
	assertDictKeys(t, list.Levels[0], []string{"code", "created_at", "description", "level_id", "name", "sort_order", "status"})

	rec = catalogRequest(t, r, token, "PUT", fmt.Sprintf("/api/admin/level/%d", levelID), `{"name":"初级"}`)
	_, _, data = unpackData(t, rec, http.StatusOK)
	var updated map[string]any
	if err := json.Unmarshal([]byte(data), &updated); err != nil {
		t.Fatalf("解析更新响应失败: %v", err)
	}
	if updated["name"] != "初级" {
		t.Fatalf("更新返回字段不匹配: %+v", updated)
	}

	rec = catalogRequest(t, r, token, "POST", "/api/admin/level", `{"code":"beginner","name":"重复"}`)
	_, msg, _ := unpackData(t, rec, http.StatusBadRequest)
	if msg != "课程等级编码已存在" {
		t.Fatalf("重复编码消息不符: %q", msg)
	}

	rec = catalogRequest(t, r, token, "DELETE", fmt.Sprintf("/api/admin/level/%d", levelID), "")
	unpackData(t, rec, http.StatusOK)
}

// TestCatalogContract_CertificateTemplate 证书模板端点：创建/列表/更新/删除键集一致。
func TestCatalogContract_CertificateTemplate(t *testing.T) {
	r, cfg, _ := newCatalogContractEnv(t)
	token := catalogAdminToken(t, cfg, 1)

	rec := catalogRequest(t, r, token, "POST", "/api/admin/certificate-template",
		`{"code":"CERT_1","name":"叉车培训证书","validity_days":1460,"template_url":"https://x/t.pdf"}`)
	_, _, data := unpackData(t, rec, http.StatusCreated)
	var created map[string]any
	if err := json.Unmarshal([]byte(data), &created); err != nil {
		t.Fatalf("解析创建响应失败: %v", err)
	}
	assertDictKeys(t, created, []string{"code", "created_at", "description", "id", "name",
		"status", "template_url", "updated_at", "validity_days"})
	if created["code"] != "CERT_1" || created["validity_days"] != float64(1460) ||
		created["template_url"] != "https://x/t.pdf" || created["status"] != float64(1) {
		t.Fatalf("创建返回字段不匹配: %+v", created)
	}
	tplID := int(created["id"].(float64))

	// 默认有效期 365（不传 validity_days）
	rec = catalogRequest(t, r, token, "POST", "/api/admin/certificate-template", `{"code":"CERT_2","name":"默认"}`)
	_, _, data = unpackData(t, rec, http.StatusCreated)
	if err := json.Unmarshal([]byte(data), &created); err != nil {
		t.Fatalf("解析创建响应失败: %v", err)
	}
	if created["validity_days"] != float64(365) {
		t.Fatalf("默认有效期应为 365: %+v", created)
	}

	// 无效有效期
	rec = catalogRequest(t, r, token, "POST", "/api/admin/certificate-template", `{"code":"C3","name":"x","validity_days":0}`)
	_, msg, _ := unpackData(t, rec, http.StatusBadRequest)
	if msg != "证书有效期必须为正整数（天）" {
		t.Fatalf("无效有效期消息不符: %q", msg)
	}

	rec = catalogRequest(t, r, token, "GET", "/api/admin/certificate-templates", "")
	_, _, data = unpackData(t, rec, http.StatusOK)
	var list struct {
		Templates []map[string]any `json:"certificate_templates"`
	}
	if err := json.Unmarshal([]byte(data), &list); err != nil || len(list.Templates) != 2 {
		t.Fatalf("模板列表解析失败: %v, data=%s", err, data)
	}
	assertDictKeys(t, list.Templates[0], []string{"code", "created_at", "description", "id", "name",
		"status", "template_url", "updated_at", "validity_days"})

	rec = catalogRequest(t, r, token, "PUT", fmt.Sprintf("/api/admin/certificate-template/%d", tplID),
		`{"validity_days":730}`)
	_, _, data = unpackData(t, rec, http.StatusOK)
	var updated map[string]any
	if err := json.Unmarshal([]byte(data), &updated); err != nil {
		t.Fatalf("解析更新响应失败: %v", err)
	}
	if updated["validity_days"] != float64(730) {
		t.Fatalf("更新返回字段不匹配: %+v", updated)
	}

	rec = catalogRequest(t, r, token, "DELETE", fmt.Sprintf("/api/admin/certificate-template/%d", tplID), "")
	unpackData(t, rec, http.StatusOK)
}

// TestCatalogContract_QuestionTag 题库标签端点 + 题目打标：键集一致。
func TestCatalogContract_QuestionTag(t *testing.T) {
	r, cfg, deps := newCatalogContractEnv(t)
	token := catalogAdminToken(t, cfg, 1)

	rec := catalogRequest(t, r, token, "POST", "/api/admin/question-tag",
		`{"code":"hydraulic","name":"液压","sort_order":1}`)
	_, _, data := unpackData(t, rec, http.StatusCreated)
	var created map[string]any
	if err := json.Unmarshal([]byte(data), &created); err != nil {
		t.Fatalf("解析创建响应失败: %v", err)
	}
	// 创建返回不含 question_count
	assertDictKeys(t, created, []string{"code", "created_at", "description", "id", "name", "sort_order", "status", "updated_at"})
	if created["code"] != "hydraulic" || created["name"] != "液压" || created["sort_order"] != float64(1) {
		t.Fatalf("创建返回字段不匹配: %+v", created)
	}
	tagID := int(created["id"].(float64))

	// 列表：question_count 恒存在（无题目时为 0）
	rec = catalogRequest(t, r, "", "GET", "/api/tags", "")
	_, _, data = unpackData(t, rec, http.StatusOK)
	var list struct {
		Tags []map[string]any `json:"tags"`
	}
	if err := json.Unmarshal([]byte(data), &list); err != nil || len(list.Tags) != 1 {
		t.Fatalf("学员端标签列表解析失败: %v, data=%s", err, data)
	}
	assertDictKeys(t, list.Tags[0], []string{"code", "created_at", "description", "id", "name",
		"question_count", "sort_order", "status", "updated_at"})
	if list.Tags[0]["question_count"] != float64(0) {
		t.Fatalf("无题目标签 question_count 应为 0: %+v", list.Tags[0])
	}

	// 更新
	rec = catalogRequest(t, r, token, "PUT", fmt.Sprintf("/api/admin/question-tag/%d", tagID), `{"name":"液压系统"}`)
	_, _, data = unpackData(t, rec, http.StatusOK)
	var updated map[string]any
	if err := json.Unmarshal([]byte(data), &updated); err != nil {
		t.Fatalf("解析更新响应失败: %v", err)
	}
	assertDictKeys(t, updated, []string{"code", "created_at", "description", "id", "name", "sort_order", "status", "updated_at"})
	if updated["name"] != "液压系统" {
		t.Fatalf("更新返回字段不匹配: %+v", updated)
	}

	// 重复编码
	rec = catalogRequest(t, r, token, "POST", "/api/admin/question-tag", `{"code":"hydraulic","name":"重复"}`)
	_, msg, _ := unpackData(t, rec, http.StatusBadRequest)
	if msg != "标签编码已存在" {
		t.Fatalf("重复编码消息不符: %q", msg)
	}

	// 题目打标：seed 题目后全量替换 + 查询（响应为关联摘要，无扩展字段）
	q := testutil.SeedQuestion(t, deps.DB, "single_choice", "液压相关题目", "A")
	rec = catalogRequest(t, r, token, "PUT", fmt.Sprintf("/api/admin/question/%d/tags", q.ID),
		`{"tag_ids":[`+fmt.Sprint(tagID)+`]}`)
	_, _, data = unpackData(t, rec, http.StatusOK)
	var setResp map[string]any
	if err := json.Unmarshal([]byte(data), &setResp); err != nil {
		t.Fatalf("解析打标响应失败: %v", err)
	}
	if ids, ok := setResp["tag_ids"].([]any); !ok || len(ids) != 1 || ids[0] != float64(tagID) {
		t.Fatalf("打标响应 tag_ids 不匹配: %+v", setResp)
	}

	rec = catalogRequest(t, r, token, "GET", fmt.Sprintf("/api/admin/question/%d/tags", q.ID), "")
	_, _, data = unpackData(t, rec, http.StatusOK)
	var tagsResp struct {
		Tags []map[string]any `json:"tags"`
	}
	if err := json.Unmarshal([]byte(data), &tagsResp); err != nil || len(tagsResp.Tags) != 1 {
		t.Fatalf("题目标签查询解析失败: %v, data=%s", err, data)
	}
	assertDictKeys(t, tagsResp.Tags[0], []string{"code", "id", "name", "sort_order", "status"})
	if tagsResp.Tags[0]["id"] != float64(tagID) || tagsResp.Tags[0]["name"] != "液压系统" {
		t.Fatalf("题目标签查询结果不匹配: %+v", tagsResp.Tags[0])
	}

	// 列表 question_count 随题目数变化（管理端全量）
	rec = catalogRequest(t, r, token, "GET", "/api/admin/question-tags", "")
	_, _, data = unpackData(t, rec, http.StatusOK)
	if err := json.Unmarshal([]byte(data), &list); err != nil || len(list.Tags) != 1 {
		t.Fatalf("管理端标签列表解析失败: %v, data=%s", err, data)
	}
	if list.Tags[0]["question_count"] != float64(1) {
		t.Fatalf("打标后 question_count 应为 1: %+v", list.Tags[0])
	}

	// 删除
	rec = catalogRequest(t, r, token, "DELETE", fmt.Sprintf("/api/admin/question-tag/%d", tagID), "")
	unpackData(t, rec, http.StatusOK)
}
