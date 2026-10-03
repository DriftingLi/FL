// 错题本域的 DTO shape-lock：从 internal/service 的 stats_aggregate_dto_test.go 与
// envelope_dto_shape_test.go 随域包搬来（ADR-0070 波 4c），断言逐字保持（只去掉包限定名）。
package wrongquestion

import (
	"encoding/json"
	"testing"
)

func TestWrongQuestionStatsDTOShapeLock(t *testing.T) {
	// 旧 wrong_question GetStats 顶层：{total, by_type}
	d := WrongQuestionStatsDTO{
		Total:  3,
		ByType: map[string]int64{"single_choice": 2, "true_false": 1},
	}
	assertShapeLock(t, d, "total", "by_type")
}

// TestEnvelopeDTOShapeLock 信封 DTO 的 shape-lock（spec #940 片三口径）：
// 左边是收口前的 map 形态、右边是 typed DTO，json.Marshal 结果必须逐字节相等。
func TestEnvelopeDTOShapeLock(t *testing.T) {
	cases := []struct {
		name   string
		legacy any
		dto    any
	}{
		{
			name: "WrongQuestionPageDTO",
			legacy: map[string]any{
				"total":     int64(1),
				"page":      1,
				"page_size": 20,
				"items": []map[string]any{
					{
						"id": 9, "student_id": 1, "question_id": 42, "wrong_count": 3,
						"last_wrong_at":    "2026-09-13T10:00:00.000000+08:00",
						"last_user_answer": "B",
						"is_removed":       false, "is_redone": false,
						"created_at":  "2026-09-13T09:00:00.000000+08:00",
						"favorited":   true,
						"favorite_id": int64(5),
					},
				},
			},
			dto: &WrongQuestionPageDTO{
				Items: []WrongQuestionDTO{{
					CreatedAt:      "2026-09-13T09:00:00.000000+08:00",
					FavoriteID:     5,
					Favorited:      true,
					ID:             9,
					IsRedone:       false,
					IsRemoved:      false,
					LastUserAnswer: "B",
					LastWrongAt:    "2026-09-13T10:00:00.000000+08:00",
					QuestionID:     42,
					StudentID:      1,
					WrongCount:     3,
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
				"last_wrong_at": "", "last_user_answer": "", "is_removed": false, "is_redone": false,
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
