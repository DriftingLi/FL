package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"forklift-training/internal/config"
	"forklift-training/internal/model"
	"forklift-training/internal/security"
	"forklift-training/internal/testutil"
)

// 证件作用域契约（ADR-0047 §4 / spec #931）：事实源在服务端。
//
// 锁三件事：
//  1. 客户端**不传** credential_id 时，端点按用户当前证件过滤（改造前会静默返回全量）；
//  2. 显式传参优先（保留「浏览其他证件」的既有用法）；
//  3. 未选证件时退回「不分区」（与改造前匿名/无证件行为一致）。
//
// 主 seam：HTTP + 真实 JWT（现有契约测试的同一层）；被测端点是题库统计（受作用域、按证件分区）。
func TestCredentialScopeContract(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := testutil.NewMemoryDB(t)
	cfg := &config.Config{JWTSecretKey: "credential-scope-secret"}
	r := NewRouter(newContractDeps(t, db, cfg))

	credA := &model.Credential{Code: "n1", Name: "叉车司机N1", Category: "special_operation", Status: 1}
	credB := &model.Credential{Code: "l5", Name: "工程机械维修工L5", Category: "skill_level", Status: 1}
	if err := db.Create(credA).Error; err != nil {
		t.Fatalf("建证件失败: %v", err)
	}
	if err := db.Create(credB).Error; err != nil {
		t.Fatalf("建证件失败: %v", err)
	}
	for i, cred := range []*model.Credential{credA, credA, credB, credB, credB} {
		q := testutil.SeedQuestion(t, db, "single", "题干", "答案")
		if err := db.Model(q).Update("credential_id", cred.ID).Error; err != nil {
			t.Fatalf("题目挂证件 %d 失败: %v", i, err)
		}
	}

	student := testutil.SeedStudent(t, db, "scope_student", "x")
	if err := db.Model(student).Update("current_credential_id", credA.ID).Error; err != nil {
		t.Fatalf("设置当前证件失败: %v", err)
	}
	tok, err := security.NewSession(cfg.JWTSecretKey, time.Hour, security.CookieConfig{}).Issue(int(student.ID), student.Account, "hrwai_user")
	if err != nil {
		t.Fatalf("签发 token 失败: %v", err)
	}

	statsTotal := func(path string) int {
		t.Helper()
		req, _ := http.NewRequest(http.MethodGet, path, nil)
		req.Header.Set("Authorization", "Bearer "+tok)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("GET %s → %d %s", path, w.Code, w.Body.String())
		}
		var raw map[string]any
		if err := json.Unmarshal(w.Body.Bytes(), &raw); err != nil {
			t.Fatalf("解析响应失败: %v", err)
		}
		data, ok := raw["data"].(map[string]any)
		if !ok {
			t.Fatalf("响应缺少 data: %s", w.Body.String())
		}
		total, ok := data["total"].(float64)
		if !ok {
			t.Fatalf("响应缺少 total: %s", w.Body.String())
		}
		return int(total)
	}
	itoa := func(v int) string { return strconv.Itoa(v) }

	if got := statsTotal("/api/question-bank/stats"); got != 2 {
		t.Fatalf("不传 credential_id 应按服务端当前证件（N1）过滤：total=%d, want 2", got)
	}
	if got := statsTotal("/api/question-bank/stats?credential_id=" + itoa(credB.ID)); got != 3 {
		t.Fatalf("显式 credential_id 应优先：total=%d, want 3", got)
	}

	// 未选证件：退回不分区（既有行为）
	if err := db.Model(student).Update("current_credential_id", nil).Error; err != nil {
		t.Fatalf("清空当前证件失败: %v", err)
	}
	if got := statsTotal("/api/question-bank/stats"); got != 5 {
		t.Fatalf("未选证件应不分区：total=%d, want 5", got)
	}
	// 未选证件时显式传参仍生效
	if got := statsTotal("/api/question-bank/stats?credential_id=" + itoa(credA.ID)); got != 2 {
		t.Fatalf("未选证件时显式传参仍生效：total=%d, want 2", got)
	}
}

// 非学员角色不得被证件作用域过滤（ADR-0047 §4 / 代码审查发现）：admin / tutor / recruiter 的 JWT sub
// 来自各自的表，而 current_credential_id 是 hrwai_users 的列——按 sub 直查会命中**同号的陌生学员行**，
// 让控制台请求被静默按别人的证件过滤。改造前客户端也从不给这三端注入证件，故语义是「非学员 = 不分区」。
func TestCredentialScopeIgnoresNonStudentRoles(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := testutil.NewMemoryDB(t)
	cfg := &config.Config{JWTSecretKey: "credential-scope-secret"}
	r := NewRouter(newContractDeps(t, db, cfg))

	credA := &model.Credential{Code: "n1b", Name: "叉车司机N1", Category: "special_operation", Status: 1}
	if err := db.Create(credA).Error; err != nil {
		t.Fatalf("建证件失败: %v", err)
	}
	for i := 0; i < 3; i++ {
		q := testutil.SeedQuestion(t, db, "single", "题干", "答案")
		if err := db.Model(q).Update("credential_id", credA.ID).Error; err != nil {
			t.Fatalf("题目挂证件失败: %v", err)
		}
	}
	// 学员的当前证件刻意设为**没有题目**的 credB：这样「被误按学员证件过滤」会得到 0，
	// 与「不分区」的 3 形成可判别差异（否则两种路径都得 3，用例退化成语义不变式）。
	credB := &model.Credential{Code: "l5b", Name: "工程机械维修工L5", Category: "skill_level", Status: 1}
	if err := db.Create(credB).Error; err != nil {
		t.Fatalf("建证件失败: %v", err)
	}
	student := testutil.SeedStudent(t, db, "scope_student2", "x")
	if err := db.Model(student).Update("current_credential_id", credB.ID).Error; err != nil {
		t.Fatalf("设置当前证件失败: %v", err)
	}
	// 讲师 token：sub 与学员同号（正是审查指出的同号命中场景）——JWTAuth 只校验 token 本身，
	// 不查 tutor 行，故这里直接签发即可，无需建讲师记录。
	tutorTok, err := security.NewSession(cfg.JWTSecretKey, time.Hour, security.CookieConfig{}).Issue(int(student.ID), "scope_tutor", "tutor")
	if err != nil {
		t.Fatalf("签发讲师 token 失败: %v", err)
	}

	req, _ := http.NewRequest(http.MethodGet, "/api/question-bank/stats", nil)
	req.Header.Set("Authorization", "Bearer "+tutorTok)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("讲师访问题库统计 → %d %s", w.Code, w.Body.String())
	}
	var raw map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &raw); err != nil {
		t.Fatalf("解析响应失败: %v", err)
	}
	data, _ := raw["data"].(map[string]any)
	total, _ := data["total"].(float64)
	if int(total) != 3 {
		t.Fatalf("讲师不应被证件作用域过滤（同号学员行不得命中）：total=%v, want 3", total)
	}
}
