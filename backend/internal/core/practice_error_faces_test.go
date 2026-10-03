// 第②批 B 段（练习域）的**事实→错误**档位锁（ADR-0064 决策 8 的 service 侧形态）。
//
// 为什么这一批改在 service 层打台账、而不是像第①②批 A 段那样走 HTTP：
// 这几个端点全部挂着能力位 + 证件分区中间件（CapQuestionPractice + CredentialScoped），
// HTTP 档要造的是「角色×能力×证件×种子数据」的笛卡尔积，而**分档这件事的判据本身住在域包**：
// 哪个事实发哪个错误；api 侧那张表只是把它映射成码。
//
// 模考与真题两段（TestMockExamNotFoundIsOnlyForMissingRows / TestRealPaperThreeFacts）
// 已随域包搬去 internal/mockexam/error_faces_test.go 与 internal/realexam/error_faces_test.go（波 4a）。
//
// 修前的形状（逐条由下面的断言钉住）：
//   - 抽题 `if err != nil { return errors.New("查询题目失败") }` ×3 —— 吞掉真实错误，
//     再被端点那格 errStatusAll(404) 渲染成 404 ⇒ 「数据库查不动」对外表现为「题库没有题」。
//   - 「请指定题库标签」「该标签不支持专项练习」同为 404 ⇒ 请求本身不成立被说成资源不存在。
//   - practice SubmitAnswer 的「题目不存在」是同文案的第二载体（questionbank.ErrQuestionNotFound 早已存在），
//     且同样不分成因。
package core

import (
	"errors"
	"testing"

	"go.uber.org/zap"

	"forklift-training/internal/model"
	"forklift-training/internal/practicemode"
	"forklift-training/internal/questionbank"
	"forklift-training/internal/testutil"
)

// TestPracticeFailureIsNotLaundered 抽题查不动必须是「服务端故障」，不是「没有题」。
// 断言打在错误身份上：不得再是那条自造的「查询题目失败」文案，而要原样上抛底层错误。
func TestPracticeFailureIsNotLaundered(t *testing.T) {
	for _, tc := range []struct {
		name string
		call func(s *practicemode.Service) error
	}{
		{"GetFreeQuestions", func(s *practicemode.Service) error {
			_, err := s.GetFreeQuestions("", 5, nil)
			return err
		}},
		{"StartSequential", func(s *practicemode.Service) error {
			_, err := s.StartSequential(1, nil)
			return err
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db := testutil.NewMemoryDB(t)
			svc := practicemode.NewService(db, nil, zap.NewNop())
			if err := db.Exec("DROP TABLE question").Error; err != nil {
				t.Fatalf("注入故障（删 question 表）失败: %v", err)
			}
			err := tc.call(svc)
			if err == nil {
				t.Fatal("抽题查不动却返回成功")
			}
			if errors.Is(err, questionbank.ErrQuestionNotFound) {
				t.Fatalf("查不动被打扮成「题目不存在」: %v", err)
			}
			if msg := err.Error(); msg == "查询题目失败" {
				t.Fatalf("仍在自造文案里吞掉真实错误（对外会被渲染成 404）: %s", msg)
			}
		})
	}
}

// TestStartTagPracticeInputFacesAreNamed 标签练习的两条入参约束各自具名 ——
// 它们此前与「资源不存在」共用 404。
func TestStartTagPracticeInputFacesAreNamed(t *testing.T) {
	db := testutil.NewMemoryDB(t)
	svc := practicemode.NewService(db, nil, zap.NewNop())

	if _, err := svc.StartTagPractice(1, 0, 5, nil); !errors.Is(err, practicemode.ErrPracticeTagRequired) {
		t.Fatalf("缺标签应报具名 practicemode.ErrPracticeTagRequired，实际 %v", err)
	}

	src := model.QuestionTag{Name: "源标记标签", IsSourceTag: true}
	if err := db.Create(&src).Error; err != nil {
		t.Fatalf("播种真题源标签失败: %v", err)
	}
	if _, err := svc.StartTagPractice(1, src.ID, 5, nil); !errors.Is(err, practicemode.ErrPracticeTagUnsupported) {
		t.Fatalf("源标记标签应报具名 practicemode.ErrPracticeTagUnsupported，实际 %v", err)
	}
}

// TestPracticeSubmitUsesExistingQuestionCarrier 「题目不存在」早有一个具名载体
// （questionbank.ErrQuestionNotFound，笔记/评论读路径都在用），practice 侧此前又写了一遍同文案裸错误
// ⇒ 同一个事实两个住处（ADR-0064 决策 2）。
func TestPracticeSubmitUsesExistingQuestionCarrier(t *testing.T) {
	db := testutil.NewMemoryDB(t)
	svc := practicemode.NewService(db, nil, zap.NewNop())

	_, err := svc.SubmitAnswer(1, 999999, "A", "free", nil)
	if !errors.Is(err, questionbank.ErrQuestionNotFound) {
		t.Fatalf("应复用既有载体 questionbank.ErrQuestionNotFound，实际 %v", err)
	}
	if err != nil && err.Error() == "题目不存在" && !errors.Is(err, questionbank.ErrQuestionNotFound) {
		t.Fatalf("同文案第二载体又出现了: %v", err)
	}
}
