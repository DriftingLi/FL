package service

import (
	"testing"

	"forklift-training/internal/model"
)

func TestCheckAnswer_SingleChoice(t *testing.T) {
	q := &model.Question{Type: "single_choice", Answer: "A"}
	// 正确
	r := checkAnswer(q, "A")
	if r == nil || !*r {
		t.Error("单选 A 应判定正确")
	}
	// 错误
	r = checkAnswer(q, "B")
	if r == nil || *r {
		t.Error("单选 B 应判定错误")
	}
	// 大小写不敏感
	r = checkAnswer(q, "a")
	if r == nil || !*r {
		t.Error("单选 a 应判定正确（大小写不敏感）")
	}
}

func TestCheckAnswer_TrueFalse(t *testing.T) {
	q := &model.Question{Type: "true_false", Answer: "TRUE"}
	r := checkAnswer(q, "true")
	if r == nil || !*r {
		t.Error("判断 true 应判定正确")
	}
	r = checkAnswer(q, "false")
	if r == nil || *r {
		t.Error("判断 false 应判定错误")
	}
}

func TestCheckAnswer_MultiChoice(t *testing.T) {
	q := &model.Question{Type: "multi_choice", Answer: "A,B,C"}
	// 完全正确
	r := checkAnswer(q, []interface{}{"A", "B", "C"})
	if r == nil || !*r {
		t.Error("多选 ABC 应判定正确")
	}
	// 顺序无关
	r = checkAnswer(q, []interface{}{"C", "A", "B"})
	if r == nil || !*r {
		t.Error("多选 CAB 应判定正确（顺序无关）")
	}
	// 不完全正确
	r = checkAnswer(q, []interface{}{"A", "B"})
	if r == nil || *r {
		t.Error("多选 AB 应判定错误")
	}
}

func TestCheckAnswer_ShortAnswer(t *testing.T) {
	q := &model.Question{Type: "short_answer", Answer: "参考答案"}
	r := checkAnswer(q, "任何答案")
	if r != nil {
		t.Error("简答题应返回 nil（无法判定）")
	}
}

func TestCheckAnswer_NilAnswer(t *testing.T) {
	q := &model.Question{Type: "single_choice", Answer: "A"}
	r := checkAnswer(q, nil)
	if r != nil {
		t.Error("未作答应返回 nil")
	}
}

func TestGradeQuestion_SingleChoice(t *testing.T) {
	q := &model.Question{Type: "single_choice", Answer: "A"}
	// 正确得满分
	correct, score := gradeQuestion(q, "A", 3)
	if correct == nil || !*correct || score != 3 {
		t.Errorf("单选正确: correct=%v score=%v，期望 true 3", correct, score)
	}
	// 错误得 0 分
	correct, score = gradeQuestion(q, "B", 3)
	if correct == nil || *correct || score != 0 {
		t.Errorf("单选错误: correct=%v score=%v，期望 false 0", correct, score)
	}
}

func TestGradeQuestion_MultiChoice_Correct(t *testing.T) {
	q := &model.Question{Type: "multi_choice", Answer: "A,B,C"}
	correct, score := gradeQuestion(q, []interface{}{"A", "B", "C"}, 4)
	if correct == nil || !*correct || score != 4 {
		t.Errorf("多选全对: correct=%v score=%v，期望 true 4", correct, score)
	}
}

func TestGradeQuestion_MultiChoice_Partial(t *testing.T) {
	q := &model.Question{Type: "multi_choice", Answer: "A,B,C"}
	// 部分正确（子集）得 50% 按比例分
	correct, score := gradeQuestion(q, []interface{}{"A", "B"}, 4)
	if correct == nil || *correct {
		t.Errorf("多选部分对: correct=%v，期望 false", correct)
	}
	// 2/3 * 4 * 0.5 = 1.33... → round1 → 1.3
	if score <= 0 {
		t.Errorf("多选部分对应有分，得到 %v", score)
	}
}

func TestGradeQuestion_MultiChoice_Wrong(t *testing.T) {
	q := &model.Question{Type: "multi_choice", Answer: "A,B,C"}
	// 包含错误选项
	correct, score := gradeQuestion(q, []interface{}{"A", "D"}, 4)
	if correct == nil || *correct {
		t.Errorf("多选含错项: correct=%v，期望 false", correct)
	}
	if score != 0 {
		t.Errorf("多选含错项应 0 分，得到 %v", score)
	}
}

func TestGradeQuestion_TrueFalse(t *testing.T) {
	q := &model.Question{Type: "true_false", Answer: "TRUE"}
	correct, score := gradeQuestion(q, "true", 2)
	if correct == nil || !*correct || score != 2 {
		t.Errorf("判断正确: correct=%v score=%v", correct, score)
	}
	correct, score = gradeQuestion(q, "false", 2)
	if correct == nil || *correct || score != 0 {
		t.Errorf("判断错误: correct=%v score=%v", correct, score)
	}
}

func TestGradeQuestion_ShortAnswer(t *testing.T) {
	q := &model.Question{Type: "short_answer", Answer: "答案"}
	correct, score := gradeQuestion(q, "学员答案", 5)
	if correct != nil {
		t.Error("简答题 correct 应为 nil")
	}
	if score != 0 {
		t.Errorf("简答题 score 应为 0，得到 %v", score)
	}
}

func TestGradeQuestion_NilAnswer(t *testing.T) {
	q := &model.Question{Type: "single_choice", Answer: "A"}
	correct, score := gradeQuestion(q, nil, 3)
	if correct != nil {
		t.Error("未作答 correct 应为 nil")
	}
	if score != 0 {
		t.Errorf("未作答 score 应为 0，得到 %v", score)
	}
}

func TestNormalizeAnswerList(t *testing.T) {
	result := normalizeAnswerList("B,A,C")
	if len(result) != 3 || result[0] != "A" || result[1] != "B" || result[2] != "C" {
		t.Errorf("normalizeAnswerList 排序失败: %v", result)
	}
	// 含空格
	result = normalizeAnswerList(" A , B , C ")
	if len(result) != 3 || result[0] != "A" {
		t.Errorf("含空格处理失败: %v", result)
	}
}

func TestStringSliceEqual(t *testing.T) {
	if !stringSliceEqual([]string{"A", "B"}, []string{"A", "B"}) {
		t.Error("相同切片应返回 true")
	}
	if stringSliceEqual([]string{"A", "B"}, []string{"A", "C"}) {
		t.Error("不同切片应返回 false")
	}
	if stringSliceEqual([]string{"A"}, []string{"A", "B"}) {
		t.Error("不同长度应返回 false")
	}
}

func TestSubset(t *testing.T) {
	if !subset([]string{"A", "B"}, []string{"A", "B", "C"}) {
		t.Error("子集应返回 true")
	}
	if subset([]string{"A", "D"}, []string{"A", "B", "C"}) {
		t.Error("非子集应返回 false")
	}
	if !subset([]string{}, []string{"A"}) {
		t.Error("空集是任何集合的子集")
	}
}

func TestRound1(t *testing.T) {
	f := 1.25
	round1(&f)
	if f != 1.3 {
		t.Errorf("round1(1.25) = %v，期望 1.3", f)
	}
	f = 1.24
	round1(&f)
	if f != 1.2 {
		t.Errorf("round1(1.24) = %v，期望 1.2", f)
	}
}
