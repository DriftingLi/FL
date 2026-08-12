// Package service 答题会话 module 测试：守卫、进度重建、答案三态初始化、状态展示语义。
package service

import (
	"encoding/json"
	"testing"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"forklift-training/internal/model"
	"forklift-training/internal/testutil"
)

func TestGuardOwnedInProgress(t *testing.T) {
	msgs := struct{ denied, notInProgress string }{"无权操作", "考试不在进行中"}

	if err := guardOwnedInProgress(1, "in_progress", 1, msgs.denied, msgs.notInProgress); err != nil {
		t.Errorf("本人+进行中应通过: %v", err)
	}
	if err := guardOwnedInProgress(2, "in_progress", 1, msgs.denied, msgs.notInProgress); err == nil {
		t.Error("他人会话应拒绝")
	}
	if err := guardOwnedInProgress(1, "submitted", 1, msgs.denied, msgs.notInProgress); err == nil {
		t.Error("非进行中应拒绝")
	}
}

func TestLoadOrderedQuestions_PreservesOrder(t *testing.T) {
	db := testutil.NewMemoryDB(t)
	q1 := testutil.SeedQuestion(t, db, "single_choice", "题1", "A")
	q2 := testutil.SeedQuestion(t, db, "single_choice", "题2", "A")
	q3 := testutil.SeedQuestion(t, db, "single_choice", "题3", "A")

	// 保存顺序与 id 顺序相反，返回必须按保存顺序
	ordered, qMap := loadOrderedQuestions(db, []int{q3.ID, q1.ID, q2.ID, 9999})
	if len(ordered) != 3 {
		t.Fatalf("应返回 3 题（缺失 id 跳过）, got %d", len(ordered))
	}
	if ordered[0].ID != q3.ID || ordered[1].ID != q1.ID || ordered[2].ID != q2.ID {
		t.Fatalf("顺序必须与保存顺序一致: %+v", ordered)
	}
	if _, ok := qMap[q1.ID]; !ok {
		t.Error("qMap 应可按下标取题")
	}
	if _, ok := qMap[9999]; ok {
		t.Error("qMap 不应包含缺失题")
	}
}

func TestAnswersMapRoundTrip(t *testing.T) {
	if got := answersMapRoundTrip(model.JSONB(nil)); len(got) != 0 {
		t.Errorf("nil JSONB 应归一为空 map, got %v", got)
	}
	if got := answersMapRoundTrip(model.JSONB([]byte("null"))); len(got) != 0 {
		t.Errorf("JSON null 应归一为空 map, got %v", got)
	}
	got := answersMapRoundTrip(model.JSONB([]byte(`{"5":"A"}`)))
	if got["5"] != "A" {
		t.Errorf("round-trip 失败: %v", got)
	}
}

// TestInitAnswersState 回归锁定 #142：显式 null/空/缺失三态一律落库为 {}，
// 不允许 JSONB 'null' 写库造成 SQL NULL。
func TestInitAnswersState(t *testing.T) {
	cases := []json.RawMessage{nil, {}, []byte("null"), []byte("{}"), []byte(`{"1":"A"}`)}
	for _, c := range cases {
		out := initAnswersState(c)
		var v any
		if err := json.Unmarshal(out, &v); err != nil {
			t.Fatalf("initAnswersState(%q) 输出非法 JSON: %v", c, err)
		}
		m, ok := v.(map[string]any)
		if !ok {
			t.Fatalf("initAnswersState(%q) 应为 JSON 对象, got %q", c, out)
		}
		if len(c) > 0 && string(c) == `{"1":"A"}` {
			if len(m) != 1 {
				t.Fatalf("有内容时不得清空: %q", out)
			}
		}
	}
}

func TestEffectiveExamStatus(t *testing.T) {
	start := time.Date(2026, 8, 1, 9, 0, 0, 0, time.UTC)
	end := start.Add(2 * time.Hour)

	cases := []struct {
		status string
		now    time.Time
		want   string
	}{
		{"upcoming", start.Add(-time.Hour), "upcoming"},
		{"upcoming", start.Add(time.Hour), "ongoing"},
		{"upcoming", end.Add(time.Hour), "finished"},
		{"ongoing", end.Add(time.Hour), "finished"},
		{"ongoing", start.Add(time.Hour), "ongoing"},
		{"finished", start.Add(time.Hour), "finished"},
	}
	for _, c := range cases {
		if got := effectiveExamStatus(c.status, start, end, c.now); got != c.want {
			t.Errorf("effectiveExamStatus(%s, %v) = %s, want %s", c.status, c.now, got, c.want)
		}
	}
}

// TestLevelExamReadPathsDoNotWrite GET 读路径不得写库（语义修正锁定）。
func TestLevelExamReadPathsDoNotWrite(t *testing.T) {
	db := testutil.NewMemoryDB(t)
	svc := NewLevelExamService(db, nil, zap.NewNop())

	start := time.Now().Add(-time.Hour)
	end := time.Now().Add(time.Hour)
	sess := model.ExamSession{
		Name: "已开始场次", StartTime: start, EndTime: end,
		Duration: 90, Status: "upcoming", TotalScore: 100, PassScore: 60,
		CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	if err := db.Create(&sess).Error; err != nil {
		t.Fatal(err)
	}

	// 可用列表：展示为 ongoing，但 DB 状态不得被改写
	available, err := svc.GetAvailableExams(1)
	if err != nil {
		t.Fatal(err)
	}
	if len(available) != 1 || available[0].Status != "ongoing" {
		t.Fatalf("可用列表应展示 ongoing, got %+v", available)
	}
	assertSessionStatusInDB(t, db, sess.ID, "upcoming")

	// 管理端列表：同样只展示、不写库
	list := svc.ListSessions(1, 20, "", false)
	if len(list.Sessions) != 1 || list.Sessions[0].Status != "ongoing" {
		t.Fatalf("管理端列表应展示 ongoing, got %+v", list.Sessions)
	}
	assertSessionStatusInDB(t, db, sess.ID, "upcoming")

	// 场次详情：展示基于时间的生效状态，不写库
	detail, err := svc.GetSessionDetail(sess.ID)
	if err != nil {
		t.Fatal(err)
	}
	if detail.Status != "ongoing" {
		t.Fatalf("场次详情应展示 ongoing, got %s", detail.Status)
	}
	assertSessionStatusInDB(t, db, sess.ID, "upcoming")
}

// TestEnterExamAdvancesSession 写路径语义：学员进入考试时 upcoming→ongoing 落库。
func TestEnterExamAdvancesSession(t *testing.T) {
	db := testutil.NewMemoryDB(t)
	svc := NewLevelExamService(db, nil, zap.NewNop())

	start := time.Now().Add(-time.Hour)
	end := time.Now().Add(time.Hour)
	sess := model.ExamSession{
		Name: "场次", StartTime: start, EndTime: end,
		Duration: 90, Status: "upcoming", TotalScore: 100, PassScore: 60,
		CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	if err := db.Create(&sess).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := svc.EnterExam(sess.ID, 1); err != nil {
		t.Fatalf("进入考试失败: %v", err)
	}
	assertSessionStatusInDB(t, db, sess.ID, "ongoing")

	// 开始时间前进入仍被拒绝（状态不推进）
	future := model.ExamSession{
		Name: "未开始场次", StartTime: time.Now().Add(2 * time.Hour), EndTime: time.Now().Add(4 * time.Hour),
		Duration: 90, Status: "upcoming", TotalScore: 100, PassScore: 60,
		CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	if err := db.Create(&future).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := svc.EnterExam(future.ID, 2); err == nil {
		t.Fatal("开始时间前进入应被拒绝")
	}
	assertSessionStatusInDB(t, db, future.ID, "upcoming")
}

func assertSessionStatusInDB(t *testing.T, db *gorm.DB, id int, want string) {
	t.Helper()
	var stored model.ExamSession
	if err := db.First(&stored, id).Error; err != nil {
		t.Fatal(err)
	}
	if stored.Status != want {
		t.Fatalf("DB 状态应为 %s, got %s", want, stored.Status)
	}
}
