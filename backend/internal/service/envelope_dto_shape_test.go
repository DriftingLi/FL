package service

import (
	"encoding/json"
	"testing"
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
