// Package service 真题套卷测试：全池隔离、按卷练习/考试、按套兑换。
package service

import (
	"context"
	"strconv"
	"testing"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"forklift-training/internal/clock"
	"forklift-training/internal/model"
	"forklift-training/internal/testutil"
)

func newRealExamSvc(t *testing.T) (*RealExamService, *PointsService, *QuestionBankService, *gorm.DB) {
	t.Helper()
	db := testutil.NewMemoryDB(t)
	points := NewPointsService(db, zap.NewNop(), clock.Real())
	qsvc := NewQuestionBankService(db, nil, zap.NewNop())
	return NewRealExamService(db, points, zap.NewNop()), points, qsvc, db
}

func itoa(n int) string { return strconv.Itoa(n) }

// seedPaper 建证件 + 卷 + 卷题关联，返回 (paperID, 卷内题目按卷序的 ID)。
func seedPaper(t *testing.T, db *gorm.DB, qsvc *QuestionBankService, qContents ...string) (int, []int) {
	t.Helper()
	cred := &model.Credential{Code: "forklift_n1", Name: "叉车司机N1证", Category: "special_operation"}
	if err := db.Create(cred).Error; err != nil {
		t.Fatalf("建证件失败: %v", err)
	}
	paper := &model.RealExamPaper{
		CredentialID:    cred.ID,
		Title:           "2026年叉车司机N1真题（全国通用）",
		DurationMinutes: 90,
		SourceRef:       "N1/001_2026年叉车司机N1证考试真题（全国通用）.md",
		QuestionCount:   len(qContents),
		Status:          1,
	}
	if err := db.Create(paper).Error; err != nil {
		t.Fatalf("建卷失败: %v", err)
	}
	ids := make([]int, 0, len(qContents))
	for i, c := range qContents {
		q, err := qsvc.CreateQuestion(map[string]any{
			"type": "single_choice", "content": c,
			"options": []string{"A", "B"}, "answer": "A", "status": "published",
		}, nil, "tutor")
		if err != nil {
			t.Fatalf("建题失败: %v", err)
		}
		ids = append(ids, q.ID)
		if err := db.Create(&model.RealExamPaperQuestion{PaperID: paper.PaperID, QuestionID: q.ID, OrderNum: i + 1}).Error; err != nil {
			t.Fatalf("建卷题关联失败: %v", err)
		}
	}
	return paper.PaperID, ids
}

// entitle 直接写入权益（绕过兑换扣分流程，模拟已兑换状态）。
func entitle(t *testing.T, db *gorm.DB, userID, paperID int) {
	t.Helper()
	if err := db.Create(&model.UserEntitlement{UserID: userID, SKU: RealPaperSKU(paperID), RefID: itoa(paperID)}).Error; err != nil {
		t.Fatalf("写权益失败: %v", err)
	}
}

func TestRealPaperPoolIsolation(t *testing.T) {
	_, _, qsvc, db := newRealExamSvc(t)
	catalogSvc := NewTrainingCatalogService(db, zap.NewNop())

	srcTag, _ := catalogSvc.CreateQuestionTag(QuestionTagInput{Code: "real_exam", Name: "真题"})
	if err := db.Model(&model.QuestionTag{}).Where("id = ?", srcTag.ID).Update("is_source_tag", true).Error; err != nil {
		t.Fatalf("置 source 标签失败: %v", err)
	}
	normalTag, _ := catalogSvc.CreateQuestionTag(QuestionTagInput{Code: "regulation", Name: "法规"})

	// 真题题（source 标签）+ 普通题
	if _, err := qsvc.CreateQuestion(map[string]any{
		"type": "single_choice", "content": "真题独有题", "options": []string{"A", "B"}, "answer": "A",
		"status": "published", "tag_ids": []int{srcTag.ID},
	}, nil, "tutor"); err != nil {
		t.Fatalf("建真题题失败: %v", err)
	}
	if _, err := qsvc.CreateQuestion(map[string]any{
		"type": "single_choice", "content": "普通题", "options": []string{"A", "B"}, "answer": "A",
		"status": "published", "tag_ids": []int{normalTag.ID},
	}, nil, "tutor"); err != nil {
		t.Fatalf("建普通题失败: %v", err)
	}

	// 随机/专项抽题池不含真题题
	psvc := NewPracticeModeService(db, nil, zap.NewNop())
	free, err := psvc.GetFreeQuestions("", 0)
	if err != nil {
		t.Fatalf("随机抽题失败: %v", err)
	}
	for _, q := range free {
		if q.Content == "真题独有题" {
			t.Fatal("真题题不应出现在随机抽题池")
		}
	}
	seq, err := psvc.StartSequential(1)
	if err != nil {
		t.Fatalf("顺序练习失败: %v", err)
	}
	for _, q := range seq.Questions {
		if q.Content == "真题独有题" {
			t.Fatal("真题题不应出现在顺序练习")
		}
	}

	// 学员端标签列表不出现 source 标签；管理端保留
	studentTags := catalogSvc.ListQuestionTags(true, false)
	for _, tg := range studentTags {
		if tg.Code == "real_exam" {
			t.Fatal("学员端标签列表不应出现真题标签")
		}
	}
	adminTags := catalogSvc.ListQuestionTags(true, true)
	found := false
	for _, tg := range adminTags {
		if tg.Code == "real_exam" {
			found = true
		}
	}
	if !found {
		t.Fatal("管理端标签列表应保留真题标签")
	}

	// StartTagPractice 对 source 标签直接拒绝
	if _, err := psvc.StartTagPractice(1, srcTag.ID, 0); err == nil {
		t.Fatal("source 标签应拒绝专项练习")
	}
}

func TestRealPaperPractice(t *testing.T) {
	svc, _, qsvc, db := newRealExamSvc(t)
	paperID, qIDs := seedPaper(t, db, qsvc, "卷题一", "卷题二", "卷题三")

	// 未兑换拒绝
	if _, err := svc.StartPaperPractice(1, paperID); err == nil {
		t.Fatal("未兑换应拒绝按卷练习")
	}

	entitle(t, db, 1, paperID)
	got, err := svc.StartPaperPractice(1, paperID)
	if err != nil {
		t.Fatalf("按卷练习失败: %v", err)
	}
	if len(got.Questions) != 3 {
		t.Fatalf("应返回 3 题, got %d", len(got.Questions))
	}
	// 卷序固定 = 关联表 order_num 序
	for i, q := range got.Questions {
		if q.ID != qIDs[i] {
			t.Fatalf("卷序不符: 位置 %d 期望 %d got %d", i, qIDs[i], q.ID)
		}
	}
	if got.CurrentIndex != 0 {
		t.Fatalf("首次进入游标应为 0, got %d", got.CurrentIndex)
	}

	// 断点续练：保存游标后再进入，从游标处恢复
	pm := NewPracticeModeService(db, nil, zap.NewNop())
	if err := pm.SaveProgress(1, 2, "paper:"+itoa(paperID), 3, nil); err != nil {
		t.Fatalf("保存进度失败: %v", err)
	}
	resumed, err := svc.StartPaperPractice(1, paperID)
	if err != nil {
		t.Fatalf("续练失败: %v", err)
	}
	if resumed.CurrentIndex != 2 {
		t.Fatalf("续练游标应为 2, got %d", resumed.CurrentIndex)
	}
}

func TestRealPaperExam(t *testing.T) {
	svc, _, qsvc, db := newRealExamSvc(t)
	paperID, qIDs := seedPaper(t, db, qsvc, "考试题一", "考试题二")

	if _, err := svc.StartPaperExam(1, paperID); err == nil {
		t.Fatal("未兑换应拒绝按卷开考")
	}

	entitle(t, db, 1, paperID)
	got, err := svc.StartPaperExam(1, paperID)
	if err != nil {
		t.Fatalf("按卷开考失败: %v", err)
	}
	if got.MockExamID == 0 || len(got.Questions) != 2 {
		t.Fatalf("开考返回异常: %+v", got)
	}
	if got.Duration != 90 || got.RemainingTime != 90*60 {
		t.Fatalf("卷时长应为 90 分钟: %+v", got)
	}
	for i, q := range got.Questions {
		if q.ID != qIDs[i] {
			t.Fatalf("考试卷序不符: 位置 %d", i)
		}
	}
	// mock_exam 记录归卷
	var mock model.MockExam
	if err := db.First(&mock, got.MockExamID).Error; err != nil {
		t.Fatalf("查 mock_exam 失败: %v", err)
	}
	if mock.PaperID == nil || *mock.PaperID != paperID {
		t.Fatalf("mock_exam.paper_id 应为 %d", paperID)
	}
	// 交卷走模拟考链路（客观题判分）
	msvc := NewMockExamService(db, nil, zap.NewNop())
	if err := msvc.SaveProgress(got.MockExamID, 1, map[string]any{itoa(qIDs[0]): "A"}, 80*60); err != nil {
		t.Fatalf("保存考试进度失败: %v", err)
	}
	if _, err := msvc.Submit(got.MockExamID, 1); err != nil {
		t.Fatalf("交卷失败: %v", err)
	}
}

func TestRedeemRealPaper(t *testing.T) {
	svc, points, qsvc, db := newRealExamSvc(t)
	paperID, _ := seedPaper(t, db, qsvc, "兑换卷题一", "兑换卷题二")

	student := testutil.SeedStudent(t, db, "redeemer", "pwd")
	if err := db.Model(&model.HrwaiUser{}).Where("id = ?", student.ID).Update("points_balance", 500).Error; err != nil {
		t.Fatalf("设余额失败: %v", err)
	}

	// 价格读商城项
	if err := db.Create(&model.PointsShopItem{SKU: "unlock_real_paper", Title: "解锁真题套卷1套", Price: 300, Enabled: true}).Error; err != nil {
		t.Fatalf("建商城项失败: %v", err)
	}

	res, err := points.RedeemRealPaper(context.Background(), student.ID, paperID)
	if err != nil {
		t.Fatalf("兑换失败: %v", err)
	}
	if res.SKU != RealPaperSKU(paperID) || res.RefID != itoa(paperID) {
		t.Fatalf("兑换 sku/ref_id 不符: %+v", res)
	}
	if res.Balance != 200 {
		t.Fatalf("扣费后余额应为 200, got %d", res.Balance)
	}
	// 幂等：重复兑换报已兑换
	if _, err := points.RedeemRealPaper(context.Background(), student.ID, paperID); err == nil || err.Error() != "已兑换" {
		t.Fatalf("重复兑换应报已兑换, got %v", err)
	}
	// 兑换后可按卷练习
	if _, err := svc.StartPaperPractice(student.ID, paperID); err != nil {
		t.Fatalf("兑换后按卷练习应成功: %v", err)
	}
	// 下架卷不可兑换
	if err := db.Model(&model.RealExamPaper{}).Where("paper_id = ?", paperID).Update("status", 0).Error; err != nil {
		t.Fatalf("下架失败: %v", err)
	}
	if _, err := points.RedeemRealPaper(context.Background(), student.ID, paperID); err == nil {
		t.Fatal("下架卷应不可兑换")
	}
	_ = svc
}
