// 模考历史按证件分区（#1003）的 HTTP 契约。
//
// 为什么必须有这一层：服务层单测只覆盖「过滤逻辑本身」，而本缺陷的形态是**接线漏了** ——
// handler 从未读当前证件（同组的 Start 是读的），服务层怎么改它都发现不了。故这里端到端锁住
// 「中间件解析当前证件 → handler 透传 → 服务层过滤」这条线，三件事：
//  1. 不传参：按服务端当前证件分区；
//  2. 显式 credential_id 优先（保留「浏览其他证件」的既有用法）；
//  3. 未选证件：退回不分区（看全部）。
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

func TestMockExamHistoryCredentialPartition(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := testutil.NewMemoryDB(t)
	cfg := &config.Config{JWTSecretKey: "mock-exam-credential-secret"}
	r := NewRouter(newContractDeps(t, db, cfg))

	credA := &model.Credential{Code: "n1h", Name: "叉车司机N1", Category: "special_operation", Status: 1}
	credB := &model.Credential{Code: "elech", Name: "低压电工", Category: "special_operation", Status: 1}
	for _, c := range []*model.Credential{credA, credB} {
		if err := db.Create(c).Error; err != nil {
			t.Fatalf("建证件失败: %v", err)
		}
	}

	student := testutil.SeedStudent(t, db, "mock_history_student", "x")
	if err := db.Model(student).Update("current_credential_id", credA.ID).Error; err != nil {
		t.Fatalf("设置当前证件失败: %v", err)
	}

	// 同一学员在两个证件下各一条已交卷记录（created_at 递进，便于断言顺序）
	base := time.Now()
	examA := model.MockExam{StudentID: student.ID, CredentialID: &credA.ID, Status: "submitted", StartTime: &base, SubmitTime: &base, CreatedAt: base}
	examB := model.MockExam{StudentID: student.ID, CredentialID: &credB.ID, Status: "submitted", StartTime: &base, SubmitTime: &base, CreatedAt: base.Add(time.Hour)}
	for _, m := range []model.MockExam{examA, examB} {
		if err := db.Create(&m).Error; err != nil {
			t.Fatalf("插入模拟考试失败: %v", err)
		}
	}
	// 插入后取回自增 ID（&m 是循环副本，需重新查）
	if err := db.Where("credential_id = ?", credB.ID).First(&examB).Error; err != nil {
		t.Fatalf("查 B 证件记录失败: %v", err)
	}
	if err := db.Where("credential_id = ?", credA.ID).First(&examA).Error; err != nil {
		t.Fatalf("查 A 证件记录失败: %v", err)
	}

	tok, err := security.NewSession(cfg.JWTSecretKey, time.Hour, security.CookieConfig{}).Issue(int(student.ID), student.Account, "hrwai_user")
	if err != nil {
		t.Fatalf("签发 token 失败: %v", err)
	}

	historyIDs := func(path string) []int {
		t.Helper()
		req, _ := http.NewRequest(http.MethodGet, path, nil)
		req.Header.Set("Authorization", "Bearer "+tok)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("GET %s → %d %s", path, w.Code, w.Body.String())
		}
		var raw struct {
			Data struct {
				Total int64 `json:"total"`
				Exams []struct {
					ID int `json:"id"`
				} `json:"exams"`
			} `json:"data"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &raw); err != nil {
			t.Fatalf("解析响应失败: %v (%s)", err, w.Body.String())
		}
		if int(raw.Data.Total) != len(raw.Data.Exams) {
			t.Fatalf("total 与 exams 数量不一致: %s", w.Body.String())
		}
		ids := make([]int, 0, len(raw.Data.Exams))
		for _, e := range raw.Data.Exams {
			ids = append(ids, e.ID)
		}
		return ids
	}
	equal := func(name string, got, want []int) {
		t.Helper()
		if len(got) != len(want) {
			t.Fatalf("%s: got %v, want %v", name, got, want)
		}
		for i := range got {
			if got[i] != want[i] {
				t.Fatalf("%s: got %v, want %v", name, got, want)
			}
		}
	}

	equal("不传参应按服务端当前证件（N1）分区", historyIDs("/api/mock-exam/history"), []int{examA.ID})
	equal("显式 credential_id 应优先", historyIDs("/api/mock-exam/history?credential_id="+strconv.Itoa(credB.ID)), []int{examB.ID})

	// 未选证件：退回不分区（两条都回，按 created_at DESC）
	if err := db.Model(student).Update("current_credential_id", nil).Error; err != nil {
		t.Fatalf("清空当前证件失败: %v", err)
	}
	equal("未选证件应不分区（看全部）", historyIDs("/api/mock-exam/history"), []int{examB.ID, examA.ID})
}
