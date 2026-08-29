// Package service AI 解析 module 测试：缓存命中 / miss 同步生成回写 / 生成失败与未配置降级。
// 期望值为独立字面量；fake 生成器记录调用，验证「练习与错题重做共用同一入口」的策略单点。
package service

import (
	"errors"
	"testing"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"forklift-training/internal/model"
	"forklift-training/internal/testutil"
)

// fakeExplGen 可注入的解析生成 adapter，记录调用参数并按预设结果返回。
type fakeExplGen struct {
	content string
	err     error
	called  int
	gotQA   string
}

func (f *fakeExplGen) GenerateQuestionExplanation(questionContent, answer, explanation string) (string, error) {
	f.called++
	f.gotQA = questionContent
	return f.content, f.err
}

func newExplModule(t *testing.T, db *gorm.DB, gen ExplanationGenerator) *QuestionExplanation {
	t.Helper()
	return &QuestionExplanation{db: db, gen: gen, logger: zap.NewNop()}
}

func TestGetOrGenerate_CacheHit(t *testing.T) {
	db := testutil.NewMemoryDB(t)
	q := testutil.SeedQuestion(t, db, "single_choice", "缓存题", "A")
	db.Model(&model.Question{}).Where("id = ?", q.ID).Update("ai_explanation", "缓存的解析")
	db.First(q, q.ID)

	gen := &fakeExplGen{content: "不应被调用"}
	m := newExplModule(t, db, gen)

	if got := m.GetOrGenerate(q); got != "缓存的解析" {
		t.Fatalf("缓存命中应直回, got %q", got)
	}
	if gen.called != 0 {
		t.Fatalf("缓存命中不应触发生成器, called=%d", gen.called)
	}
}

func TestGetOrGenerate_MissGeneratesAndPersists(t *testing.T) {
	db := testutil.NewMemoryDB(t)
	q := testutil.SeedQuestion(t, db, "single_choice", "首答题", "A")

	gen := &fakeExplGen{content: "新生成的解析"}
	m := newExplModule(t, db, gen)

	if got := m.GetOrGenerate(q); got != "新生成的解析" {
		t.Fatalf("miss 应返回生成内容, got %q", got)
	}
	if gen.called != 1 || gen.gotQA != "首答题" {
		t.Fatalf("应恰好调用一次生成器: called=%d qa=%q", gen.called, gen.gotQA)
	}
	var cached string
	db.Model(&model.Question{}).Where("id = ?", q.ID).Select("ai_explanation").Scan(&cached)
	if cached != "新生成的解析" {
		t.Fatalf("生成内容应回写缓存列, got %q", cached)
	}
}

func TestGetOrGenerate_GenErrorFallsBackStatic(t *testing.T) {
	db := testutil.NewMemoryDB(t)
	q := testutil.SeedQuestion(t, db, "single_choice", "失败题", "A")
	db.Model(&model.Question{}).Where("id = ?", q.ID).Update("explanation", "静态解析")
	db.First(q, q.ID)

	gen := &fakeExplGen{err: errors.New("llm down")}
	m := newExplModule(t, db, gen)

	if got := m.GetOrGenerate(q); got != "静态解析" {
		t.Fatalf("生成失败应降级静态解析, got %q", got)
	}
	var cached string
	db.Model(&model.Question{}).Where("id = ?", q.ID).Select("ai_explanation").Scan(&cached)
	if cached != "" {
		t.Fatalf("失败不得污染缓存列, got %q", cached)
	}
}

func TestGetOrGenerate_NilGenFallsBackStatic(t *testing.T) {
	db := testutil.NewMemoryDB(t)
	q := testutil.SeedQuestion(t, db, "single_choice", "未配置题", "A")
	db.Model(&model.Question{}).Where("id = ?", q.ID).Update("explanation", "静态解析")
	db.First(q, q.ID)

	m := newExplModule(t, db, nil)
	if got := m.GetOrGenerate(q); got != "静态解析" {
		t.Fatalf("未配置生成器应降级静态解析, got %q", got)
	}
}
