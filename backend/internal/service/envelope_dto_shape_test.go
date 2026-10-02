package service

import (
	"encoding/json"
	"testing"

	"forklift-training/internal/aiassistant"
	"forklift-training/internal/notification"
	"forklift-training/internal/points"
	"forklift-training/internal/practicemode"
	"forklift-training/internal/questionbank"
	"forklift-training/internal/questioninteraction"
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
		// TutorRegisterResultDTO 的用例已随域包搬去 internal/auth/（ADR-0070 波 3a）。
		{
			name: "questionbank.QuestionPageDTO",
			legacy: map[string]any{
				"total":     int64(3),
				"page":      1,
				"page_size": 20,
				"questions": []questionbank.QuestionDTO{},
			},
			dto: &questionbank.QuestionPageDTO{Page: 1, PageSize: 20, Questions: []questionbank.QuestionDTO{}, Total: 3},
		},
		{
			name:   "questionbank.QuestionPublishResultDTO",
			legacy: map[string]any{"published_count": 2},
			dto:    &questionbank.QuestionPublishResultDTO{PublishedCount: 2},
		},
		{
			name:   "questionbank.QuestionRejectResultDTO",
			legacy: map[string]any{"rejected_count": 1},
			dto:    &questionbank.QuestionRejectResultDTO{RejectedCount: 1},
		},
		{
			name: "questionbank.QuestionImportResultDTO（errors 内层 key 序也要一致）",
			legacy: map[string]any{
				"success_count": 1,
				"error_count":   1,
				"errors": []map[string]any{
					{"index": 2, "error": "无效数据"},
				},
			},
			dto: &questionbank.QuestionImportResultDTO{
				ErrorCount:   1,
				Errors:       []questionbank.QuestionImportErrorDTO{{Error: "无效数据", Index: 2}},
				SuccessCount: 1,
			},
		},
		{
			name: "questionbank.QuestionImportResultDTO（无失败时是 [] 不是 null）",
			legacy: map[string]any{
				"success_count": 2,
				"error_count":   0,
				"errors":        []map[string]any{},
			},
			dto: &questionbank.QuestionImportResultDTO{ErrorCount: 0, Errors: []questionbank.QuestionImportErrorDTO{}, SuccessCount: 2},
		},
		// WrongQuestion* 的五条用例已随域包搬去 internal/wrongquestion/dto_shape_test.go（ADR-0070 波 4c）。
		// WechatQRCodeInfoDTO 的用例已随域包搬去 internal/auth/（ADR-0070 波 3a）。
		{
			// #1095：ContactRequestListResult 从「只服务 swagger 的类型」变真返回类型。
			// 左边是改造前 handler 里的 gin.H（map 按 key 排序），右边是 typed page —— 字节必须相等。
			name: "ContactRequestListResult（旧 gin.H ↔ typed page，键序 items/page/page_size/total）",
			legacy: map[string]any{
				"items":     []ContactRequestDTO{},
				"page":      1,
				"page_size": 20,
				"total":     int64(0),
			},
			dto: &ContactRequestListResult{Items: []ContactRequestDTO{}, Page: 1, PageSize: 20, Total: 0},
		},
		// ForumTopicDetailDTO 的信封形状用例已随域包搬去 internal/forum/（ADR-0070 波 2b-2）。
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
	// created / updated 与 rec 夹具随 RecruiterCreatedDTO/RecruiterUpdatedDTO 用例搬去 internal/auth/。
	// HrwaiUserCreatedDTO / StatusResultDTO 两枚 DTO 及其三例已随管理域搬去 internal/admin/（ADR-0070 波 4d）。

	cases := []struct {
		name   string
		legacy any
		dto    any
	}{
		// RecruiterCreatedDTO / RecruiterUpdatedDTO 的用例已随域包搬去 internal/auth/（ADR-0070 波 3a）。
		// StatusResultDTO ×2 与 HrwaiUserCreatedDTO 的用例已随管理域搬去 internal/admin/envelope_dto_shape_test.go（ADR-0070 波 4d）。
		{
			name:   "GenerateContentResultDTO",
			legacy: map[string]any{"task_id": "task-abc"},
			dto:    &GenerateContentResultDTO{TaskID: "task-abc"},
		},
		{
			name:   "points.PointsPenaltyResultDTO",
			legacy: map[string]any{"deducted": 30},
			dto:    &points.PointsPenaltyResultDTO{Deducted: 30},
		},
		{
			name:   "RecruitMeDTO（GET /api/recruit/me，原裸 handler）",
			legacy: map[string]any{"user_id": 9, "account": "hr009", "role": "recruiter"},
			dto:    &RecruitMeDTO{UserID: 9, Account: "hr009", Role: "recruiter"},
		},
		// RecruiterPasswordResetResult 的用例已随域包搬去 internal/auth/（ADR-0070 波 3a）。
		{
			name:   "practicemode.ProgressSaveResultDTO（POST /practice-mode/progress：原 handler 内联 map）",
			legacy: map[string]any{"saved": true, "index": 5},
			dto:    &practicemode.ProgressSaveResultDTO{Index: 5, Saved: true},
		},
		// ForumImageUploadResultDTO / ForumLikeResultDTO 的信封形状用例已随域包搬去 internal/forum/（ADR-0070 波 2b-2）。
		{
			name:   "notification.NotificationUnreadCountDTO（GET /notifications/unread-count：原 handler 内联 gin.H）",
			legacy: map[string]any{"count": int64(3)},
			dto:    &notification.NotificationUnreadCountDTO{Count: 3},
		},
		{
			name: "questioninteraction.QuestionCommentPageResult（GET /questions/{id}/comments：原 handler 内联 gin.H）",
			legacy: map[string]any{
				"items": []questioninteraction.QuestionCommentDTO{{ID: 1, Content: "这题易错"}},
				"total": int64(1), "page": 1, "page_size": 10,
			},
			dto: &questioninteraction.QuestionCommentPageResult{
				Items: []questioninteraction.QuestionCommentDTO{{ID: 1, Content: "这题易错"}},
				Page:  1, PageSize: 10, Total: 1,
			},
		},
		// RefreshResultDTO（/api/auth/refresh）的用例已随域包搬去 internal/auth/（ADR-0070 波 3a）。
		{
			name:   "aiassistant.AISessionRenameResultDTO（PATCH /ai-assistant/sessions/{id}/title：原 handler 内联 map[string]string）",
			legacy: map[string]string{"message": "已更新会话标题"},
			dto:    &aiassistant.AISessionRenameResultDTO{Message: "已更新会话标题"},
		},
		{
			name:   "aiassistant.AIImageUploadResultDTO（POST /ai-assistant/upload-image：原 handler 内联 gin.H{url}）",
			legacy: map[string]any{"url": "https://cdn.test/images/ai-assistant/chat_1.png"},
			dto:    &aiassistant.AIImageUploadResultDTO{URL: "https://cdn.test/images/ai-assistant/chat_1.png"},
		},
		// ===== ADR-0048 片六（#964）：培训目录 / 题库 / 证件域的 handler 内联 map 收口 =====
		{
			name:   "questionbank.QuestionImageUploadDTO（POST /question-bank/upload-image）",
			legacy: map[string]any{"url": "/uploads/images/questions/1.png"},
			dto:    &questionbank.QuestionImageUploadDTO{URL: "/uploads/images/questions/1.png"},
		},
		// 培训目录 / 目标证件 / 岗位面的 13 例（QuestionTagsResultDTO / LevelListDTO / QuestionTagListDTO /
		// CertificateTemplateListDTO / CredentialListDTO / CurrentCredentialDTO / SpecialtyListDTO /
		// PositionListDTO / GroupedCredentialsDTO）已随域包搬去 internal/training/envelope_dto_shape_test.go（ADR-0070 波 3b-2）。
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

// spec #966 片八：内联**匿名结构体**（不是 map）定型为命名类型后的同一套字节锁。
//
// swag 对匿名嵌套对象只吐内联 object，而渲染规则对「带 properties 的 object」只给
// { [key: string]: unknown } —— 前端 metadata.source_url 会退化成 unknown，故必须命名
// （service.aiassistant.DiagnosisSourceMetadata）。命名不得改动任何字节：字段序 / json tag / 可空性都不动。
func TestDiagnosisSourceMetadataShapeLock(t *testing.T) {
	type legacyDiagnosisSource struct {
		ID       aiassistant.DiagnosisSourceID `json:"id"`
		Text     string                        `json:"text"`
		Metadata struct {
			SourceURL string `json:"source_url"`
			PageStart int    `json:"page_start"`
			PageEnd   int    `json:"page_end"`
		} `json:"metadata"`
	}
	legacy := legacyDiagnosisSource{ID: "fault-15", Text: "手册第 3 页"}
	legacy.Metadata.SourceURL = "https://example.com/manual/x.pdf"
	legacy.Metadata.PageStart, legacy.Metadata.PageEnd = 3, 4
	dto := aiassistant.DiagnosisSource{ID: "fault-15", Text: "手册第 3 页"}
	dto.Metadata.SourceURL = "https://example.com/manual/x.pdf"
	dto.Metadata.PageStart, dto.Metadata.PageEnd = 3, 4

	want, err := json.Marshal(legacy)
	if err != nil {
		t.Fatalf("marshal legacy: %v", err)
	}
	got, err := json.Marshal(dto)
	if err != nil {
		t.Fatalf("marshal dto: %v", err)
	}
	if string(want) != string(got) {
		t.Fatalf("字节不一致（字段序 / tag 漂移）：\nlegacy = %s\ndto    = %s", want, got)
	}
}
