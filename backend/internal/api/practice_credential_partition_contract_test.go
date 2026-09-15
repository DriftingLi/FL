// 练习族读面的证件分区契约（#1007 / ADR-0051）。
//
// 为什么必须有这一层：服务层单测只覆盖「过滤逻辑本身」，而本次缺陷的形态是**接线漏了** ——
// /practice-mode/history 与 /practice-mode/stats 的 handler 从未读当前证件（同组的 submit /
// practice-stats 是读 middleware.CredentialIDPtr 的），服务层怎么改它都发现不了。故这里端到端锁住
// 「中间件解析当前证件 → handler 透传 → 服务层过滤」这条线，三个读面 × 三种读法：
//  1. 不传参：按服务端当前证件分区；
//  2. 显式 credential_id 优先（保留「浏览其他证件」的既有用法）；
//  3. 未选证件：退回不分区（看全部）。
//
// 同时断开口径对账不变式：同一证件下 practice-stats.total_count == history.total == stats.total
// —— 生产实测的缺陷形态正是 0 ≠ 15。
package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"forklift-training/internal/config"
	"forklift-training/internal/model"
	"forklift-training/internal/security"
	"forklift-training/internal/testutil"
)

func TestPracticeCredentialPartitionContract(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := testutil.NewMemoryDB(t)
	cfg := &config.Config{JWTSecretKey: "practice-credential-contract"}
	r := NewRouter(newContractDeps(t, db, cfg))

	credA := &model.Credential{Code: "partA", Name: "叉车司机N1", Category: "special_operation", Status: 1}
	credB := &model.Credential{Code: "partB", Name: "低压电工", Category: "special_operation", Status: 1}
	for _, c := range []*model.Credential{credA, credB} {
		if err := db.Create(c).Error; err != nil {
			t.Fatalf("建证件失败: %v", err)
		}
	}
	qA := testutil.SeedQuestion(t, db, "single", "A 证题", "A")
	qB := testutil.SeedQuestion(t, db, "single", "B 证题", "A")

	student := testutil.SeedStudent(t, db, "practice_partition_student", "x")
	if err := db.Model(student).Update("current_credential_id", credA.ID).Error; err != nil {
		t.Fatalf("设置当前证件失败: %v", err)
	}

	// 直接插记录（读面契约聚焦「按记录上的分区过滤」，写入路径由服务层用例覆盖）
	base := time.Now()
	recA1 := model.QuestionPracticeRecord{StudentID: student.ID, CredentialID: &credA.ID, QuestionID: qA.ID, IsCorrect: true, PracticeType: "free", CreatedAt: base}
	recA2 := model.QuestionPracticeRecord{StudentID: student.ID, CredentialID: &credA.ID, QuestionID: qA.ID, IsCorrect: false, PracticeType: "redo", CreatedAt: base.Add(time.Minute)}
	recB := model.QuestionPracticeRecord{StudentID: student.ID, CredentialID: &credB.ID, QuestionID: qB.ID, IsCorrect: true, PracticeType: "free", CreatedAt: base.Add(2 * time.Minute)}
	for _, rec := range []*model.QuestionPracticeRecord{&recA1, &recA2, &recB} {
		if err := db.Create(rec).Error; err != nil {
			t.Fatalf("插入练习记录失败: %v", err)
		}
	}

	tok, err := security.NewSession(cfg.JWTSecretKey, time.Hour, security.CookieConfig{}).Issue(int(student.ID), student.Account, "hrwai_user")
	if err != nil {
		t.Fatalf("签发 token 失败: %v", err)
	}

	get := func(path string) map[string]any {
		t.Helper()
		req, _ := http.NewRequest(http.MethodGet, path, nil)
		req.Header.Set("Authorization", "Bearer "+tok)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("GET %s → %d %s", path, w.Code, w.Body.String())
		}
		var raw struct {
			Data map[string]any `json:"data"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &raw); err != nil {
			t.Fatalf("解析 %s 响应失败: %v (%s)", path, err, w.Body.String())
		}
		return raw.Data
	}
	num := func(data map[string]any, key string) int {
		t.Helper()
		v, ok := data[key].(float64)
		if !ok {
			t.Fatalf("响应缺少 %s: %+v", key, data)
		}
		return int(v)
	}
	// 同一证件下的三个读面必须给同一个数（口径对账不变式）
	// credParam 形如 "" 或 "credential_id=N"；按各端点是否已有 query 追加分隔符。
	assertAligned := func(name, credParam string, want int) {
		t.Helper()
		withCred := func(base string) string {
			if credParam == "" {
				return base
			}
			if strings.Contains(base, "?") {
				return base + "&" + credParam
			}
			return base + "?" + credParam
		}
		hist := num(get(withCred("/api/practice-mode/history?page=1&page_size=20")), "total")
		overview := num(get(withCred("/api/practice-mode/practice-stats")), "total_count")
		stats := num(get(withCred("/api/practice-mode/stats")), "total")
		if hist != want || overview != want || stats != want {
			t.Fatalf("%s（%s）: 三读面口径不一致 history=%d practice-stats=%d stats=%d, want %d",
				name, credParam, hist, overview, stats, want)
		}
	}

	assertAligned("不传参=服务端当前证件 A", "", 2)
	assertAligned("显式 B", "credential_id="+strconv.Itoa(credB.ID), 1)
	// 显式参数与隐式当前证件指向同一证件时结果相同
	assertAligned("显式 A", "credential_id="+strconv.Itoa(credA.ID), 2)

	// 未选证件 → 退回不分区（看全部）
	if err := db.Model(student).Update("current_credential_id", nil).Error; err != nil {
		t.Fatalf("清空当前证件失败: %v", err)
	}
	assertAligned("未选证件=不分区", "", 3)
}
