// Package service 投递即授权收口测试（ADR-0027 C5）：
// EnsureApproved 单点锁定三分支状态迁移（pending 覆盖 / 新建 approved / revoked 复活），
// 行为与投递事务内联迁移完全一致。
package service

import (
	"testing"
	"time"

	"gorm.io/gorm"

	"forklift-training/internal/model"
	"forklift-training/internal/testutil"
)

// TestEnsureApproved_NoPending_NewApproved 无 pending/revoked：新建一条 approved（source=application）。
func TestEnsureApproved_NoPending_NewApproved(t *testing.T) {
	db := testutil.NewMemoryDB(t)
	svc := NewContactService(db, nil, nil, nil)
	now := time.Date(2026, 9, 4, 10, 0, 0, 0, time.Local)

	err := db.Transaction(func(tx *gorm.DB) error {
		return svc.EnsureApproved(tx, 101, 202, "学员投递职位「叉车维修工」产生的联系方式授权", now)
	})
	if err != nil {
		t.Fatalf("EnsureApproved 失败: %v", err)
	}

	var req model.ContactRequest
	if err := db.Where("recruiter_id = ? AND student_user_id = ?", 101, 202).First(&req).Error; err != nil {
		t.Fatalf("查询授权失败: %v", err)
	}
	if req.Status != "approved" || req.Source != "application" {
		t.Fatalf("应新建 approved/source=application，得到 %+v", req)
	}
	if req.Message != "学员投递职位「叉车维修工」产生的联系方式授权" {
		t.Fatalf("附言不符: %s", req.Message)
	}
	if !req.ExpiresAt.Equal(now.Add(14 * 24 * time.Hour)) {
		t.Fatalf("ExpiresAt 应为 now+14 天，得到 %v", req.ExpiresAt)
	}
	if req.DecidedAt == nil || !req.DecidedAt.Equal(now) {
		t.Fatalf("DecidedAt 应为 now，得到 %v", req.DecidedAt)
	}
}

// TestEnsureApproved_PendingOverwrite 已有 pending：覆盖为 approved（source=application），不新建。
func TestEnsureApproved_PendingOverwrite(t *testing.T) {
	db := testutil.NewMemoryDB(t)
	svc := NewContactService(db, nil, nil, nil)
	now := time.Date(2026, 9, 4, 10, 0, 0, 0, time.Local)

	seed := model.ContactRequest{
		RecruiterID: 101, StudentUserID: 202, Message: "原待决申请",
		Status: "pending", Source: "recruiter", CreatedAt: now.Add(-time.Hour), UpdatedAt: now.Add(-time.Hour),
	}
	if err := db.Create(&seed).Error; err != nil {
		t.Fatalf("seed pending 失败: %v", err)
	}

	err := db.Transaction(func(tx *gorm.DB) error {
		return svc.EnsureApproved(tx, 101, 202, "投递授权消息", now)
	})
	if err != nil {
		t.Fatalf("EnsureApproved 失败: %v", err)
	}

	var reqs []model.ContactRequest
	if err := db.Where("recruiter_id = ? AND student_user_id = ?", 101, 202).Find(&reqs).Error; err != nil {
		t.Fatalf("查询失败: %v", err)
	}
	if len(reqs) != 1 {
		t.Fatalf("应仅覆盖原 pending 不新建，得到 %d 条", len(reqs))
	}
	if reqs[0].Status != "approved" || reqs[0].Source != "application" {
		t.Fatalf("原 pending 应覆盖为 approved/source=application，得到 %+v", reqs[0])
	}
	if reqs[0].DecidedAt == nil || !reqs[0].DecidedAt.Equal(now) {
		t.Fatalf("DecidedAt 应为 now，得到 %v", reqs[0].DecidedAt)
	}
}

// TestEnsureApproved_RevokedRevive 已 revoked 的授权：复活为 approved（source 保持原值不覆盖）。
func TestEnsureApproved_RevokedRevive(t *testing.T) {
	db := testutil.NewMemoryDB(t)
	svc := NewContactService(db, nil, nil, nil)
	now := time.Date(2026, 9, 4, 10, 0, 0, 0, time.Local)
	decided := now.Add(-48 * time.Hour)

	seed := model.ContactRequest{
		RecruiterID: 101, StudentUserID: 202, Message: "旧授权",
		Status: "revoked", Source: "application", CreatedAt: now.Add(-72 * time.Hour),
		UpdatedAt: now.Add(-48 * time.Hour), DecidedAt: &decided,
	}
	if err := db.Create(&seed).Error; err != nil {
		t.Fatalf("seed revoked 失败: %v", err)
	}

	err := db.Transaction(func(tx *gorm.DB) error {
		return svc.EnsureApproved(tx, 101, 202, "重新投递授权", now)
	})
	if err != nil {
		t.Fatalf("EnsureApproved 失败: %v", err)
	}

	var req model.ContactRequest
	if err := db.Where("id = ?", seed.ID).First(&req).Error; err != nil {
		t.Fatalf("查询失败: %v", err)
	}
	if req.Status != "approved" {
		t.Fatalf("revoked 应复活为 approved，得到 %s", req.Status)
	}
	// 复活不覆盖 source（与投递内联迁移语义一致）
	if req.Source != "application" {
		t.Fatalf("复活不应覆盖 source，得到 %s", req.Source)
	}
	if req.DecidedAt == nil || !req.DecidedAt.Equal(now) {
		t.Fatalf("复活 DecidedAt 应为 now，得到 %v", req.DecidedAt)
	}
}
