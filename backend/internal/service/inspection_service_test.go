// #1097 巡检读面归位 service：typed DTO 的字节锁 + 错误上抛。
package service

import (
	"encoding/json"
	"testing"
	"time"

	"forklift-training/internal/model"
	"forklift-training/internal/testutil"
	"forklift-training/pkg/paging"
)

// TestInspectionDTOShapeLock 搬出裸 model 的字节锁（ADR-0009 §2）：
// 左边是搬迁前的出参形态（裸 model / 内联 gin.H map），右边是 typed DTO —— json.Marshal 必须逐字节相等。
// 字段声明序写错、加 omitempty、空切片写成 nil，这里立刻红。
func TestInspectionDTOShapeLock(t *testing.T) {
	viewedAt := time.Date(2026, 9, 17, 10, 0, 0, 0, time.UTC)
	createdAt := time.Date(2026, 9, 16, 9, 30, 0, 0, time.UTC)
	updatedAt := time.Date(2026, 9, 16, 9, 45, 0, 0, time.UTC)
	decidedAt := time.Date(2026, 9, 16, 9, 40, 0, 0, time.UTC)
	expiresAt := time.Date(2026, 9, 30, 9, 45, 0, 0, time.UTC)

	legacyView := model.RecruitResumeView{ID: 11, RecruiterID: 3, ResumeUserID: 7, ViewedAt: viewedAt}
	legacyReq := model.ContactRequest{
		ID: 21, RecruiterID: 3, StudentUserID: 7, Message: "巡检测试", Status: "pending", Source: "recruiter",
		CreatedAt: createdAt, UpdatedAt: updatedAt, ExpiresAt: expiresAt,
	}
	legacyReqDecided := legacyReq
	legacyReqDecided.ID = 22
	legacyReqDecided.DecidedAt = &decidedAt

	dtoView := RecruitResumeViewDTO{ID: 11, RecruiterID: 3, ResumeUserID: 7, ViewedAt: viewedAt}
	dtoReq := ContactRequestRowDTO{
		ID: 21, RecruiterID: 3, StudentUserID: 7, Message: "巡检测试", Status: "pending", Source: "recruiter",
		CreatedAt: createdAt, UpdatedAt: updatedAt, ExpiresAt: expiresAt,
	}
	dtoReqDecided := dtoReq
	dtoReqDecided.ID = 22
	dtoReqDecided.DecidedAt = &decidedAt

	cases := []struct {
		name   string
		legacy any
		dto    any
	}{
		{
			name:   "RecruitResumeViewDTO ↔ 裸 model.RecruitResumeView",
			legacy: legacyView,
			dto:    dtoView,
		},
		{
			name:   "ContactRequestRowDTO ↔ 裸 model.ContactRequest（decided_at 缺失时整个 key 不出现）",
			legacy: legacyReq,
			dto:    dtoReq,
		},
		{
			name:   "ContactRequestRowDTO ↔ 裸 model.ContactRequest（decided_at 在时按字段序落在 expires_at 前）",
			legacy: legacyReqDecided,
			dto:    dtoReqDecided,
		},
		{
			name:   "InspectionCountDTO ↔ 旧 gin.H（count 单键）",
			legacy: map[string]any{"count": 42},
			dto:    &InspectionCountDTO{Count: 42},
		},
		{
			name:   "InspectionCountDTO（零值：键在、值 0）",
			legacy: map[string]any{"count": 0},
			dto:    &InspectionCountDTO{},
		},
		{
			name: "ItemsPage[RecruitResumeViewDTO] ↔ 旧 typed page（键序 items/page/page_size/total）",
			legacy: map[string]any{
				"items": []model.RecruitResumeView{legacyView}, "page": 1, "page_size": 20, "total": int64(1),
			},
			dto: &paging.ItemsPage[RecruitResumeViewDTO]{Items: []RecruitResumeViewDTO{dtoView}, Page: 1, PageSize: 20, Total: 1},
		},
		{
			name: "ItemsPage[ContactRequestRowDTO]（空结果：items 是 [] 不是 null）",
			legacy: map[string]any{
				"items": []model.ContactRequest{}, "page": 1, "page_size": 20, "total": int64(0),
			},
			dto: &paging.ItemsPage[ContactRequestRowDTO]{Items: []ContactRequestRowDTO{}, Page: 1, PageSize: 20, Total: 0},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			want, err := json.Marshal(tc.legacy)
			if err != nil {
				t.Fatalf("marshal legacy: %v", err)
			}
			got, err := json.Marshal(tc.dto)
			if err != nil {
				t.Fatalf("marshal dto: %v", err)
			}
			if string(want) != string(got) {
				t.Fatalf("字节不一致（字段序 / omitempty / 空切片语义漂移）：\nlegacy = %s\ndto    = %s", want, got)
			}
		})
	}
}

// TestInspectionServiceQueryFailurePropagates 读库故障一律上抛（ADR-0056 §1）：
// 三条读路径都不能再把故障渲染成「空列表 / 0」。
func TestInspectionServiceQueryFailurePropagates(t *testing.T) {
	db := testutil.NewMemoryDB(t)
	svc := NewInspectionService(db)
	if err := db.Migrator().DropTable(&model.ContactRequest{}, &model.RecruitResumeView{}, &model.SystemSetting{}); err != nil {
		t.Fatalf("注入故障（删表）失败: %v", err)
	}

	if rows, err := svc.ListRecruitViews(InspectionViewsParams{Page: 1, PageSize: 20}); err == nil {
		t.Fatalf("ListRecruitViews 故障必须上抛，实际 rows=%v", rows)
	}
	if rows, err := svc.ListRecruitRequests(InspectionRequestsParams{Page: 1, PageSize: 20}); err == nil {
		t.Fatalf("ListRecruitRequests 故障必须上抛，实际 rows=%v", rows)
	}
	if got, err := svc.DeletedAfterAcceptedCount(); err == nil {
		t.Fatalf("DeletedAfterAcceptedCount 故障必须上抛，实际 %v", got)
	}
}

// TestInspectionDeletedAfterAcceptedMissingRowIsZero 计数行缺失（表在、行不在）= 0，不是故障。
func TestInspectionDeletedAfterAcceptedMissingRowIsZero(t *testing.T) {
	svc := NewInspectionService(testutil.NewMemoryDB(t))
	got, err := svc.DeletedAfterAcceptedCount()
	if err != nil {
		t.Fatalf("计数行缺失不应报错: %v", err)
	}
	if got == nil || got.Count != 0 {
		t.Fatalf("计数行缺失应为 0，实际 %v", got)
	}
}
