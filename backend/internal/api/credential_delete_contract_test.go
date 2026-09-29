// 证件删除阻塞的 HTTP 面（#1360 / 真实缺陷 #12）。
//
// checks.md 那条「登记表认领某个 error 作明文面载体时，要另写一条 HTTP 级断言钉那句话真发得出去」
// 在这里落地：服务层能 errors.Is 到哨兵不等于句子会出门——域表若给这条哨兵配了固定文案，
// 「该证件下仍有 N 篇投稿」就会被抹成一句话，条数丢失而测试仍绿。本文件断言的是**信封里那句话**。
package api

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"gorm.io/gorm"

	"forklift-training/internal/model"
	"forklift-training/internal/testutil"
)

// credDeleteEnvelope 解出信封的 code / message。
func credDeleteEnvelope(t *testing.T, body string) (int, string) {
	t.Helper()
	var env struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal([]byte(body), &env); err != nil {
		t.Fatalf("解析响应信封失败: %v body=%s", err, body)
	}
	return env.Code, env.Message
}

func seedCredForHTTPDelete(t *testing.T, db *gorm.DB, code string) *model.Credential {
	t.Helper()
	c := &model.Credential{Code: code, Name: "证件-" + code, Category: "special_operation", Status: 1}
	if err := db.Create(c).Error; err != nil {
		t.Fatalf("建证件失败: %v", err)
	}
	return c
}

func seedContribHTTPDelete(t *testing.T, db *gorm.DB, userID, credID int, n int) {
	t.Helper()
	for i := 0; i < n; i++ {
		row := model.UserContribution{
			UserID: userID, CredentialID: credID, Title: fmt.Sprintf("HTTP 投稿-%d", i), Intro: "夹具",
			Status: "pending", CreatedAt: time.Now(), UpdatedAt: time.Now(),
		}
		if err := db.Create(&row).Error; err != nil {
			t.Fatalf("建投稿失败: %v", err)
		}
	}
}

// TestDeleteCredentialBlockedHTTP 有投稿 ⇒ 400，且正文那句带条数的话原样发得出去。
func TestDeleteCredentialBlockedHTTP(t *testing.T) {
	r, db, token := newAdminContractEnv(t)
	student := testutil.SeedStudent(t, db, "cred_del_http", "x")
	cred := seedCredForHTTPDelete(t, db, "N1_http_block")
	seedContribHTTPDelete(t, db, student.ID, cred.ID, 2)

	rec := doWithToken(t, r, token, "DELETE", fmt.Sprintf("/api/admin/credential/%d", cred.ID), nil)
	if rec.Code != 400 {
		t.Fatalf("投稿阻塞应回 400（本仓未启用 409，renderStatus 里没有那一档），实得 %d body=%s",
			rec.Code, rec.Body.String())
	}
	code, msg := credDeleteEnvelope(t, rec.Body.String())
	if code != 400 {
		t.Fatalf("信封 code 应与 HTTP 档位一致，实得 %d", code)
	}
	if !strings.Contains(msg, "该证件下仍有 2 篇投稿") {
		t.Fatalf("正文必须带条数（那条事实的全部信息量），实得 %q", msg)
	}
	if !strings.Contains(msg, "请先迁移或下架") {
		t.Fatalf("正文要给出路，实得 %q", msg)
	}
	// 证件没被删掉（阻塞不是「先删再报」）。
	var cnt int64
	if err := db.Model(&model.Credential{}).Where("id = ?", cred.ID).Count(&cnt).Error; err != nil {
		t.Fatalf("计数失败: %v", err)
	}
	if cnt != 1 {
		t.Fatalf("被拒的删除不得动到证件，实得 %d 行", cnt)
	}
}

// TestDeleteCredentialWithProgressOnlyHTTP 只有练习进度（无投稿）⇒ 200。
// CASCADE 那一半在 SQLite 面上不可见（测试库不建外键），这里判的是端点不再拿外键冲突答 500；
// 分区真的随证件消失见 credential_delete_postgres_contract_test.go（首跑在 CI）。
func TestDeleteCredentialWithProgressOnlyHTTP(t *testing.T) {
	r, db, token := newAdminContractEnv(t)
	student := testutil.SeedStudent(t, db, "cred_del_http_ok", "x")
	cred := seedCredForHTTPDelete(t, db, "N1_http_progress")
	cid := cred.ID
	p := model.PracticeProgress{StudentID: student.ID, PracticeMode: "sequential", CredentialID: &cid,
		QuestionIDs: model.JSONB("[]"), AnswersState: model.JSONB("{}"), UpdatedAt: time.Now()}
	if err := db.Create(&p).Error; err != nil {
		t.Fatalf("建练习进度失败: %v", err)
	}

	rec := doWithToken(t, r, token, "DELETE", fmt.Sprintf("/api/admin/credential/%d", cred.ID), nil)
	if rec.Code != 200 {
		t.Fatalf("只有练习进度时删证件应 200（原缺陷：外键无删除动作 ⇒ 撞 FK 回 500），实得 %d body=%s",
			rec.Code, rec.Body.String())
	}
	if _, msg := credDeleteEnvelope(t, rec.Body.String()); msg != "证件删除成功" {
		t.Fatalf("成功文案漂移: %q", msg)
	}
}

// TestDeleteCredentialNotFoundHTTP 不存在的证件仍回 404（预检不得把它压成 400/200）。
func TestDeleteCredentialNotFoundHTTP(t *testing.T) {
	r, _, token := newAdminContractEnv(t)
	rec := doWithToken(t, r, token, "DELETE", "/api/admin/credential/999999", nil)
	if rec.Code != 404 {
		t.Fatalf("不存在的证件应回 404，实得 %d body=%s", rec.Code, rec.Body.String())
	}
}
