package service

import (
	"encoding/json"
	"testing"

	"forklift-training/internal/model"
)

// spec #940 片三（含片二）：信封 DTO 的 shape-lock。
//
// 每张表都是「收口前的 map 形态 ↔ 收口后的 typed DTO」：两者的 json.Marshal 结果必须**逐字节相等**。
// 这不是重复断言 —— encoding/json 对 map 按 key 排序输出，而 struct 按字段声明序输出，
// 所以字段顺序写错、加了 omitempty、或把空切片写成 nil，这里立刻红。
// 参照物写法沿用 question_dto_shape_test.go 的 legacy*Dict 先例。
func TestEnvelopeDTOShapeLock(t *testing.T) {
	cases := []struct {
		name   string
		legacy any
		dto    any
	}{
		{
			name: "TutorRegisterResultDTO",
			legacy: map[string]any{
				"tutor_id": 7,
				"username": "tutor1",
				"name":     "张老师",
			},
			dto: &TutorRegisterResultDTO{Name: "张老师", TutorID: 7, Username: "tutor1"},
		},
		{
			name: "QuestionPageDTO",
			legacy: map[string]any{
				"total":     int64(3),
				"page":      1,
				"page_size": 20,
				"questions": []QuestionDTO{},
			},
			dto: &QuestionPageDTO{Page: 1, PageSize: 20, Questions: []QuestionDTO{}, Total: 3},
		},
		{
			name:   "QuestionPublishResultDTO",
			legacy: map[string]any{"published_count": 2},
			dto:    &QuestionPublishResultDTO{PublishedCount: 2},
		},
		{
			name:   "QuestionRejectResultDTO",
			legacy: map[string]any{"rejected_count": 1},
			dto:    &QuestionRejectResultDTO{RejectedCount: 1},
		},
		{
			name: "QuestionImportResultDTO（errors 内层 key 序也要一致）",
			legacy: map[string]any{
				"success_count": 1,
				"error_count":   1,
				"errors": []map[string]any{
					{"index": 2, "error": "无效数据"},
				},
			},
			dto: &QuestionImportResultDTO{
				ErrorCount:   1,
				Errors:       []QuestionImportErrorDTO{{Error: "无效数据", Index: 2}},
				SuccessCount: 1,
			},
		},
		{
			name: "QuestionImportResultDTO（无失败时是 [] 不是 null）",
			legacy: map[string]any{
				"success_count": 2,
				"error_count":   0,
				"errors":        []map[string]any{},
			},
			dto: &QuestionImportResultDTO{ErrorCount: 0, Errors: []QuestionImportErrorDTO{}, SuccessCount: 2},
		},
		{
			name: "WrongQuestionPageDTO",
			legacy: map[string]any{
				"total":     int64(1),
				"page":      1,
				"page_size": 20,
				"items": []map[string]any{
					{
						"id": 9, "student_id": 1, "question_id": 42, "wrong_count": 3,
						"last_wrong_at": "2026-09-13T10:00:00.000000+08:00",
						"is_removed":    false, "is_redone": false,
						"created_at":  "2026-09-13T09:00:00.000000+08:00",
						"favorited":   true,
						"favorite_id": int64(5),
					},
				},
			},
			dto: &WrongQuestionPageDTO{
				Items: []WrongQuestionDTO{{
					CreatedAt:   "2026-09-13T09:00:00.000000+08:00",
					FavoriteID:  5,
					Favorited:   true,
					ID:          9,
					IsRedone:    false,
					IsRemoved:   false,
					LastWrongAt: "2026-09-13T10:00:00.000000+08:00",
					QuestionID:  42,
					StudentID:   1,
					WrongCount:  3,
				}},
				Page:     1,
				PageSize: 20,
				Total:    1,
			},
		},
		{
			name: "WrongQuestionDTO（question 缺失时整个 key 不出现）",
			legacy: map[string]any{
				"id": 9, "student_id": 1, "question_id": 42, "wrong_count": 3,
				"last_wrong_at": "", "is_removed": false, "is_redone": false,
				"created_at": "", "favorited": false, "favorite_id": int64(0),
			},
			dto: &WrongQuestionDTO{ID: 9, StudentID: 1, QuestionID: 42, WrongCount: 3},
		},
		{
			name:   "WrongQuestionRemoveResultDTO",
			legacy: map[string]any{"removed": true},
			dto:    &WrongQuestionRemoveResultDTO{Removed: true},
		},
		{
			name:   "WrongQuestionBatchRemoveResultDTO（同键不同类型：批量是条数）",
			legacy: map[string]any{"removed": 3},
			dto:    &WrongQuestionBatchRemoveResultDTO{Removed: 3},
		},
		{
			name: "WechatQRCodeInfoDTO",
			legacy: map[string]any{
				"enabled": false,
				"qr_url":  "",
				"message": "微信授权暂未配置，请等待开放平台配置完成后使用",
			},
			dto: &WechatQRCodeInfoDTO{Enabled: false, Message: "微信授权暂未配置，请等待开放平台配置完成后使用", QRURL: ""},
		},
		{
			name: "ForumTopicDetailDTO",
			legacy: map[string]any{
				"topic":   ForumTopicDTO{},
				"replies": []ForumReplyDTO{},
				"page":    1,
				"pages":   0,
				"total":   int64(0),
			},
			dto: &ForumTopicDetailDTO{Page: 1, Pages: 0, Replies: []ForumReplyDTO{}, Topic: ForumTopicDTO{}, Total: 0},
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
				t.Fatalf("字节不一致（字段顺序 / omitempty / 空切片语义漂移）：\nmap   = %s\nstruct= %s", want, got)
			}
		})
	}
}

// spec #954 片二：handler **内联构造**的响应 map 定型为 DTO 后的同一套字节锁。
//
// ADR-0009 当年把「handler 里内联构造的响应 map」明确划出范围（判据一节：另立片）——
// 本片就是那一片，因此沿用同一个机制与同一个参照物：左边是**改造前的 map 形态**
// （不是手抄的 JSON 字面量，否则抄错即与 DTO 同错），右边是收口后的 DTO。
func TestInlineResponseDTOBytes(t *testing.T) {
	rec := &model.RecruiterUser{
		ID: 7, Username: "hr001", CompanyName: "叉车租赁有限公司", CreditCode: "91310000MA1K3XYZ",
		BusinessScope: "叉车租赁与维修", ContactName: "王工", ContactPhone: "13800000000",
		ContactEmail: "hr@example.com", Wechat: "wx_hr001", Status: 1,
	}
	user := &model.HrwaiUser{ID: 12, UID: 20260012, Account: "hrwai012", Username: "张三", Phone: "13800000001"}
	created, updated := NewRecruiterCreatedDTO(rec), NewRecruiterUpdatedDTO(rec)
	newUser := NewHrwaiUserCreatedDTO(user)

	cases := []struct {
		name   string
		legacy any
		dto    any
	}{
		{
			name:   "StatusResultDTO（HRWAI 用户 / 导师 / 招聘者三个开关端点共用；来源 int16）",
			legacy: map[string]any{"status": int16(1)},
			dto:    &StatusResultDTO{Status: 1},
		},
		{
			name:   "StatusResultDTO（来源 int，零值也不省略）",
			legacy: map[string]any{"status": 0},
			dto:    &StatusResultDTO{},
		},
		{
			name: "RecruiterCreatedDTO（创建 201：含 status）",
			legacy: map[string]any{
				"id": rec.ID, "username": rec.Username, "company_name": rec.CompanyName,
				"credit_code": rec.CreditCode, "business_scope": rec.BusinessScope,
				"contact_name": rec.ContactName, "contact_phone": rec.ContactPhone,
				"contact_email": rec.ContactEmail, "wechat": rec.Wechat, "status": rec.Status,
			},
			dto: &created,
		},
		{
			name: "RecruiterCreatedDTO（零值：无 omitempty，10 个 key 一个不少）",
			legacy: map[string]any{
				"id": 0, "username": "", "company_name": "", "credit_code": "",
				"business_scope": "", "contact_name": "", "contact_phone": "",
				"contact_email": "", "wechat": "", "status": int16(0),
			},
			dto: &RecruiterCreatedDTO{},
		},
		{
			name: "RecruiterUpdatedDTO（编辑 200：与创建同一个投影少一个 status —— 现状差异按字节保留）",
			legacy: map[string]any{
				"id": rec.ID, "username": rec.Username, "company_name": rec.CompanyName,
				"credit_code": rec.CreditCode, "business_scope": rec.BusinessScope,
				"contact_name": rec.ContactName, "contact_phone": rec.ContactPhone,
				"contact_email": rec.ContactEmail, "wechat": rec.Wechat,
			},
			dto: &updated,
		},
		{
			name: "HrwaiUserCreatedDTO（新增 HRWAI 用户 201：password 不入响应，uid 走 FormatUID）",
			legacy: map[string]any{
				"id": user.ID, "uid": FormatUID(user.UID), "account": user.Account,
				"username": user.Username, "phone": user.Phone,
			},
			dto: &newUser,
		},
		{
			name:   "GenerateContentResultDTO",
			legacy: map[string]any{"task_id": "task-abc"},
			dto:    &GenerateContentResultDTO{TaskID: "task-abc"},
		},
		{
			name:   "PointsPenaltyResultDTO",
			legacy: map[string]any{"deducted": 30},
			dto:    &PointsPenaltyResultDTO{Deducted: 30},
		},
		{
			name:   "QuestionTagsResultDTO",
			legacy: map[string]any{"tag_ids": []int{3, 5}},
			dto:    &QuestionTagsResultDTO{TagIDs: []int{3, 5}},
		},
		{
			name:   "QuestionTagsResultDTO（无标签：nil 切片仍是 null）",
			legacy: map[string]any{"tag_ids": nil},
			dto:    &QuestionTagsResultDTO{},
		},
		{
			name:   "RecruitMeDTO（GET /api/recruit/me，原裸 handler）",
			legacy: map[string]any{"user_id": 9, "account": "hr009", "role": "recruiter"},
			dto:    &RecruitMeDTO{UserID: 9, Account: "hr009", Role: "recruiter"},
		},
		{
			name:   "RecruiterPasswordResetResult（改造前是空 map：data 必须是 {} 而不是 null）",
			legacy: map[string]any{},
			dto:    &RecruiterPasswordResetResult{},
		},
		{
			name:   "ProgressSaveResultDTO（POST /practice-mode/progress：原 handler 内联 map）",
			legacy: map[string]any{"saved": true, "index": 5},
			dto:    &ProgressSaveResultDTO{Index: 5, Saved: true},
		},
		{
			name:   "RefreshResultDTO（POST /auth/refresh：原 raw handler 内联 map[string]string）",
			legacy: map[string]string{"token": "acc-1", "refresh_token": "ref-1"},
			dto:    &RefreshResultDTO{RefreshToken: "ref-1", Token: "acc-1"},
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
				t.Fatalf("字节不一致（字段顺序 / omitempty / 空对象语义漂移）：\nmap   = %s\nstruct= %s", want, got)
			}
		})
	}
}
