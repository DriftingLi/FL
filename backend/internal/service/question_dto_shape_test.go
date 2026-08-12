package service

import (
	"encoding/json"
	"testing"

	"forklift-training/internal/model"
)

// legacyQuestionDict 历史 questionToDict 的 map 实现（shape-lock 参照物）。
// 仅用于测试冻结契约：新 typed DTO 的 JSON 输出必须与之逐字一致。
func legacyQuestionDict(q *model.Question, includeAnswer bool) map[string]any {
	var options any
	if len(q.Options) > 0 {
		_ = json.Unmarshal(q.Options, &options)
	}
	d := map[string]any{
		"id":              q.ID,
		"type":            q.Type,
		"content":         q.Content,
		"options":         options,
		"image_url":       q.ImageURL,
		"status":          q.Status,
		"reject_reason":   q.RejectReason,
		"score":           q.Score,
		"created_by":      q.CreatedBy,
		"created_by_type": q.CreatedByType,
		"created_at":      formatISO(q.CreatedAt),
		"updated_at":      formatISO(q.UpdatedAt),
	}
	if includeAnswer {
		d["answer"] = q.Answer
		d["explanation"] = q.Explanation
		d["reference_answer"] = q.ReferenceAnswer
		d["scoring_criteria"] = q.ScoringCriteria
	}
	return d
}

func sampleQuestionForShape() *model.Question {
	options, _ := json.Marshal([]map[string]string{
		{"A": "选项A"}, {"B": "选项B"}, {"C": "选项C"},
	})
	createdBy := 7
	return &model.Question{
		ID:              42,
		Type:            "multi_choice",
		Content:         "题干",
		Options:         model.JSONB(options),
		ImageURL:        "https://example.com/q.png",
		Status:          "published",
		RejectReason:    "驳回理由",
		Score:           4,
		CreatedBy:       &createdBy,
		CreatedByType:   "tutor",
		Answer:          "A,B",
		Explanation:     "解析",
		ReferenceAnswer: "参考答案",
		ScoringCriteria: "评分标准",
	}
}

// TestQuestionDTO_BytesMatchLegacy 冻结题目契约：学员侧与含答案侧都必须与历史
// map 输出逐字一致（key 集合、null/省略语义、字节序）。
func TestQuestionDTO_BytesMatchLegacy(t *testing.T) {
	q := sampleQuestionForShape()

	gotStudent, _ := json.Marshal(newQuestionDTO(q, false))
	wantStudent, _ := json.Marshal(legacyQuestionDict(q, false))
	if string(gotStudent) != string(wantStudent) {
		t.Errorf("学员侧契约漂移\n got: %s\nwant: %s", gotStudent, wantStudent)
	}

	gotAnswer, _ := json.Marshal(newQuestionDTO(q, true))
	wantAnswer, _ := json.Marshal(legacyQuestionDict(q, true))
	if string(gotAnswer) != string(wantAnswer) {
		t.Errorf("含答案侧契约漂移\n got: %s\nwant: %s", gotAnswer, wantAnswer)
	}
}

// TestQuestionDTO_BytesMatchLegacy_EmptyFields 空字段形态（options 为 null、答案为空串、
// created_by 为 null）与历史行为一致。
func TestQuestionDTO_BytesMatchLegacy_EmptyFields(t *testing.T) {
	q := &model.Question{Type: "single_choice", Status: "draft"}

	got, _ := json.Marshal(newQuestionDTO(q, false))
	want, _ := json.Marshal(legacyQuestionDict(q, false))
	if string(got) != string(want) {
		t.Errorf("空字段契约漂移\n got: %s\nwant: %s", got, want)
	}

	gotAnswer, _ := json.Marshal(newQuestionDTO(q, true))
	wantAnswer, _ := json.Marshal(legacyQuestionDict(q, true))
	if string(gotAnswer) != string(wantAnswer) {
		t.Errorf("空字段含答案契约漂移\n got: %s\nwant: %s", gotAnswer, wantAnswer)
	}
}

// TestQuestionDTO_Tags 题库管理面附加 tags 的形态：设置后出现、未设置时省略。
func TestQuestionDTO_Tags(t *testing.T) {
	q := sampleQuestionForShape()

	withTags := newQuestionDTO(q, true)
	withTags.Tags = []map[string]any{{"id": 1, "name": "法规"}}
	got, _ := json.Marshal(withTags)
	var asMap map[string]any
	if err := json.Unmarshal(got, &asMap); err != nil {
		t.Fatal(err)
	}
	if _, ok := asMap["tags"]; !ok {
		t.Error("设置 Tags 后 JSON 应包含 tags key")
	}

	withoutTags := newQuestionDTO(q, false)
	gotNoTags, _ := json.Marshal(withoutTags)
	var asMap2 map[string]any
	if err := json.Unmarshal(gotNoTags, &asMap2); err != nil {
		t.Fatal(err)
	}
	if _, ok := asMap2["tags"]; ok {
		t.Error("未设置 Tags 时 JSON 不应出现 tags key")
	}
}
