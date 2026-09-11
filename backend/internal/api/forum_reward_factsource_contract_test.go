// 契约测试 ADR-0041：论坛奖励的事实源改用**不可回退的积分流水**。
//
// 本文件钉三件旧实现做错、且既有测试未覆盖的事：
//  1. 采纳配对衰减**不能**被「取消采纳」重置（旧实现把配对数读自可回退的 accepted_reply_id，
//     每轮「采纳→取消」即可把计数打回 1，衰减永不触发 → 长期只剩 120 分/天且每天重置）。
//  2. 违规回收**覆盖认定奖励**，且触发条件是「该帖存在任一正向流水」而非「曾被采纳」——
//     否则「加精但不可被采纳」的备考经验帖整片落在回收盲区。
//  3. 删被采纳的回复**不占用该帖的回收机会**（RollbackByRef 是 ref 级一次性护栏：
//     删回复若回收，之后删整帖时帖主的 30 分再也追不回）。删帖才是奖励处置的唯一出口。
//  4. 浏览量只统计**真实浏览**（排除自帖、每人每日每帖一次）——hot 排序第三键正是它。
package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"forklift-training/internal/config"
	"forklift-training/internal/model"
	"forklift-training/internal/security"
	"forklift-training/internal/service"
	"forklift-training/internal/testutil"
)

// rewardFactsEnv 本文件各用例共用的装配：整套路由 + 四个身份。
type rewardFactsEnv struct {
	db          *gorm.DB
	r           *gin.Engine
	author      model.HrwaiUser
	answerer    model.HrwaiUser
	reader      model.HrwaiUser
	reader2     model.HrwaiUser
	authorTok   string
	answererTok string
	readerTok   string
	reader2Tok  string
	adminTok    string
}

func newRewardFactsEnv(t *testing.T) *rewardFactsEnv {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db := testutil.NewMemoryDB(t)
	cfg := &config.Config{
		JWTSecretKey: "contract-test-secret",
		AuthCookie:   config.AuthCookieConfig{Name: "hrwai_token"},
	}
	r := NewRouter(newContractDeps(t, db, cfg))

	author := model.HrwaiUser{Account: "rf_author", Phone: "13800000301", Username: "楼主", Status: 1, CreatedAt: testutil.Now()}
	answerer := model.HrwaiUser{Account: "rf_answerer", Phone: "13800000302", Username: "答主", Status: 1, CreatedAt: testutil.Now()}
	reader := model.HrwaiUser{Account: "rf_reader", Phone: "13800000303", Username: "读者甲", Status: 1, CreatedAt: testutil.Now()}
	reader2 := model.HrwaiUser{Account: "rf_reader2", Phone: "13800000304", Username: "读者乙", Status: 1, CreatedAt: testutil.Now()}
	for _, u := range []*model.HrwaiUser{&author, &answerer, &reader, &reader2} {
		if err := db.Create(u).Error; err != nil {
			t.Fatalf("创建用户失败: %v", err)
		}
	}
	adminPwd, _ := service.HashPassword("admin123")
	admin := testutil.SeedAdmin(t, db, "adminRewardFacts", adminPwd)

	issue := func(id int, account, role string) string {
		tok, err := security.NewSession(cfg.JWTSecretKey, time.Hour, security.CookieConfig{Name: cfg.AuthCookie.Name}).Issue(id, account, role)
		if err != nil {
			t.Fatalf("签发 token 失败: %v", err)
		}
		return tok
	}
	return &rewardFactsEnv{
		db: db, r: r, author: author, answerer: answerer, reader: reader, reader2: reader2,
		authorTok:   issue(author.ID, author.Account, "hrwai_user"),
		answererTok: issue(answerer.ID, answerer.Account, "hrwai_user"),
		readerTok:   issue(reader.ID, reader.Account, "hrwai_user"),
		reader2Tok:  issue(reader2.ID, reader2.Account, "hrwai_user"),
		adminTok:    issue(admin.AdminID, admin.Username, "admin"),
	}
}

// decodeID 从 201 响应里取 data.id（发帖与回复同形）。
func decodeID(t *testing.T, rec *httptest.ResponseRecorder) int64 {
	t.Helper()
	var got struct {
		Data struct {
			ID int64 `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("解析响应失败: %v (body=%s)", err, rec.Body.String())
	}
	if got.Data.ID == 0 {
		t.Fatalf("响应缺少 data.id: %s", rec.Body.String())
	}
	return got.Data.ID
}

func (e *rewardFactsEnv) balance(t *testing.T, tok string) int {
	t.Helper()
	rec := doWithToken(t, e.r, tok, http.MethodGet, "/api/points/balance", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("余额查询失败 %d %s", rec.Code, rec.Body.String())
	}
	var got struct {
		Data struct {
			Balance int `json:"balance"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("解析余额失败: %v", err)
	}
	return got.Data.Balance
}

func (e *rewardFactsEnv) createQuestion(t *testing.T, tok, title string) int64 {
	t.Helper()
	rec := doWithToken(t, e.r, tok, http.MethodPost, "/api/forum/topics",
		map[string]any{"category": "question", "title": title, "content": "求助内容"})
	if rec.Code != http.StatusCreated {
		t.Fatalf("发帖 %q 应 201，实际 %d %s", title, rec.Code, rec.Body.String())
	}
	return decodeID(t, rec)
}

func (e *rewardFactsEnv) reply(t *testing.T, tok string, topicID int64, content string) int64 {
	t.Helper()
	rec := doWithToken(t, e.r, tok, http.MethodPost, fmt.Sprintf("/api/forum/topics/%d/replies", topicID), map[string]any{"content": content})
	if rec.Code != http.StatusCreated {
		t.Fatalf("回复应 201，实际 %d %s", rec.Code, rec.Body.String())
	}
	return decodeID(t, rec)
}

func (e *rewardFactsEnv) accept(t *testing.T, tok string, topicID, replyID int64) *httptest.ResponseRecorder {
	t.Helper()
	return doWithToken(t, e.r, tok, http.MethodPost, fmt.Sprintf("/api/forum/topics/%d/accept", topicID), map[string]any{"reply_id": replyID})
}

func (e *rewardFactsEnv) rollbackRows(t *testing.T, topicID int64) []model.PointsLedger {
	t.Helper()
	var rows []model.PointsLedger
	if err := e.db.Where("reason = ? AND ref_id = ?", "rollback", fmt.Sprintf("%d", topicID)).Find(&rows).Error; err != nil {
		t.Fatalf("查询回收流水失败: %v", err)
	}
	return rows
}

// TestForumPairDecayNotResettableByStateEdits 配对衰减的事实源是不可回退的流水：
// 「采纳 → 取消采纳 → 再采纳」不得把配对计数打回起点。
func TestForumPairDecayNotResettableByStateEdits(t *testing.T) {
	e := newRewardFactsEnv(t)

	// 前 3 次采纳均为满分（配对 prior 0/1/2 都 < 3）
	var topicIDs []int64
	for i := 0; i < 3; i++ {
		top := e.createQuestion(t, e.authorTok, fmt.Sprintf("配对刷分帖%d", i))
		rid := e.reply(t, e.answererTok, top, fmt.Sprintf("回答%d", i))
		if rec := e.accept(t, e.authorTok, top, rid); rec.Code != http.StatusOK {
			t.Fatalf("第 %d 次采纳应 200，实际 %d %s", i+1, rec.Code, rec.Body.String())
		}
		topicIDs = append(topicIDs, top)
	}
	if got := e.balance(t, e.answererTok); got != 120 {
		t.Fatalf("前 3 次采纳答主应 120，实际 %d", got)
	}

	// 把流水挪到前天，绕开日封顶（答主日 3 / 楼主日 5），让本用例只测配对衰减本身
	yesterday := time.Now().Add(-48 * time.Hour)
	if err := e.db.Model(&model.PointsLedger{}).
		Where("user_id IN ?", []int{e.answerer.ID, e.author.ID}).
		Update("created_at", yesterday).Error; err != nil {
		t.Fatalf("挪动流水时间失败: %v", err)
	}

	// ★ 关键动作：取消全部采纳。旧实现的配对计数读自 accepted_reply_id，
	//   这一下会把计数打回 0（只剩当前帖），配对衰减随之失效。
	for _, top := range topicIDs {
		rec := doWithToken(t, e.r, e.authorTok, http.MethodDelete, fmt.Sprintf("/api/forum/topics/%d/accept", top), nil)
		if rec.Code != http.StatusOK {
			t.Fatalf("取消采纳应 200，实际 %d %s", rec.Code, rec.Body.String())
		}
	}

	// 第 4 次采纳：已付配对 prior=3 → 减半 20（旧实现会错发 40）
	top4 := e.createQuestion(t, e.authorTok, "配对刷分帖3")
	rid4 := e.reply(t, e.answererTok, top4, "回答3")
	before4 := e.balance(t, e.answererTok)
	if rec := e.accept(t, e.authorTok, top4, rid4); rec.Code != http.StatusOK {
		t.Fatalf("第 4 次采纳应 200，实际 %d %s", rec.Code, rec.Body.String())
	}
	if delta := e.balance(t, e.answererTok) - before4; delta != 20 {
		t.Fatalf("取消采纳不得重置配对衰减：第 4 次应 +20，实际 +%d", delta)
	}

	// 第 5 次：prior=4 → 仍减半 20
	top5 := e.createQuestion(t, e.authorTok, "配对刷分帖4")
	rid5 := e.reply(t, e.answererTok, top5, "回答4")
	before5 := e.balance(t, e.answererTok)
	if rec := e.accept(t, e.authorTok, top5, rid5); rec.Code != http.StatusOK {
		t.Fatalf("第 5 次采纳应 200，实际 %d", rec.Code)
	}
	if delta := e.balance(t, e.answererTok) - before5; delta != 20 {
		t.Fatalf("第 5 次配对应 +20，实际 +%d", delta)
	}

	// 第 6 次：prior=5 → 归零（每对终身 3×40 + 2×20 = 160 的上限）
	top6 := e.createQuestion(t, e.authorTok, "配对刷分帖5")
	rid6 := e.reply(t, e.answererTok, top6, "回答5")
	before6 := e.balance(t, e.answererTok)
	if rec := e.accept(t, e.authorTok, top6, rid6); rec.Code != http.StatusOK {
		t.Fatalf("第 6 次采纳应 200，实际 %d", rec.Code)
	}
	if delta := e.balance(t, e.answererTok) - before6; delta != 0 {
		t.Fatalf("第 6 次配对应归零，实际 +%d", delta)
	}
	if got := e.balance(t, e.answererTok); got != 160 {
		t.Fatalf("每对终身上限应 160，实际 %d", got)
	}

	fmt.Println("配对衰减事实源契约通过：取消采纳不重置计数，每对终身 160 上限守住")
}

// TestForumRollbackCoversRewardWithoutAccept 回收触发条件是「存在任一正向流水」，
// 而不是「曾被采纳」——否则「加精但不可被采纳」的备考经验帖永远追不回。
func TestForumRollbackCoversRewardWithoutAccept(t *testing.T) {
	e := newRewardFactsEnv(t)

	rec := doWithToken(t, e.r, e.authorTok, http.MethodPost, "/api/forum/topics",
		map[string]any{"category": "discussion", "title": "从未被采纳的精选帖", "content": "x"})
	if rec.Code != http.StatusCreated {
		t.Fatalf("发帖应 201，实际 %d %s", rec.Code, rec.Body.String())
	}
	topicID := decodeID(t, rec)

	if rec := doWithToken(t, e.r, e.adminTok, http.MethodPost, fmt.Sprintf("/api/admin/forum/topics/%d/featured", topicID), nil); rec.Code != http.StatusOK {
		t.Fatalf("加精应 200，实际 %d %s", rec.Code, rec.Body.String())
	}
	if got := e.balance(t, e.authorTok); got != 30 {
		t.Fatalf("加精后帖主应 +30，实际 %d", got)
	}

	// ★ 删帖必须追回：该帖从未被采纳，旧守卫（AcceptedReplyID != nil）会整片漏掉
	if rec := doWithToken(t, e.r, e.adminTok, http.MethodDelete, fmt.Sprintf("/api/admin/forum/topics/%d", topicID), nil); rec.Code != http.StatusOK {
		t.Fatalf("管理员删帖应 200，实际 %d %s", rec.Code, rec.Body.String())
	}
	if got := e.balance(t, e.authorTok); got != 0 {
		t.Fatalf("加精但未采纳的帖被删应追回 30（ADR-0041），实际余额 %d", got)
	}
	rows := e.rollbackRows(t, topicID)
	if len(rows) != 1 || rows[0].Delta != -30 {
		t.Fatalf("应恰一条 -30 回收流水，实际 %+v", rows)
	}

	fmt.Println("回收触发条件契约通过：加精但未采纳的帖被删同样追回 30")
}

// TestForumRollbackSurvivesReplyDeletion 删被采纳的回复只清采纳状态、不回收，
// 以便之后删整帖时三笔奖励仍能一次全部追回（ref 级一次性护栏）。
func TestForumRollbackSurvivesReplyDeletion(t *testing.T) {
	e := newRewardFactsEnv(t)

	top := e.createQuestion(t, e.authorTok, "先删回答再删整帖")
	rid := e.reply(t, e.answererTok, top, "违规回答")
	if rec := e.accept(t, e.authorTok, top, rid); rec.Code != http.StatusOK {
		t.Fatalf("采纳应 200，实际 %d %s", rec.Code, rec.Body.String())
	}
	if rec := doWithToken(t, e.r, e.adminTok, http.MethodPost, fmt.Sprintf("/api/admin/forum/topics/%d/featured", top), nil); rec.Code != http.StatusOK {
		t.Fatalf("加精应 200，实际 %d %s", rec.Code, rec.Body.String())
	}
	if got := e.balance(t, e.authorTok); got != 35 { // 采纳动作 +5 与认定 +30
		t.Fatalf("楼主应 35，实际 %d", got)
	}
	if got := e.balance(t, e.answererTok); got != 40 {
		t.Fatalf("答主应 40，实际 %d", got)
	}

	// 管理员先删被采纳的违规回答：只清采纳状态，不回收
	if rec := doWithToken(t, e.r, e.adminTok, http.MethodDelete, fmt.Sprintf("/api/admin/forum/replies/%d", rid), nil); rec.Code != http.StatusOK {
		t.Fatalf("管理员删回复应 200，实际 %d %s", rec.Code, rec.Body.String())
	}
	if got := e.balance(t, e.authorTok); got != 35 {
		t.Fatalf("删回复不应动楼主余额，实际 %d", got)
	}
	if got := e.balance(t, e.answererTok); got != 40 {
		t.Fatalf("删回复不应动答主余额，实际 %d", got)
	}
	if rows := e.rollbackRows(t, top); len(rows) != 0 {
		t.Fatalf("删回复不得占用该帖的回收机会（否则帖主 30 分永远追不回），实际 rollback %d 条", len(rows))
	}
	// 采纳状态必须显式清干净：accepted_reply_id 有 ON DELETE SET NULL 外键兜底，solved_at 没有
	var row model.ForumTopic
	if err := e.db.First(&row, top).Error; err != nil {
		t.Fatalf("回读主题失败: %v", err)
	}
	if row.AcceptedReplyID != nil || row.SolvedAt != nil {
		t.Fatalf("删被采纳回复后应回到未解决态：accepted_reply_id=%v solved_at=%v", row.AcceptedReplyID, row.SolvedAt)
	}

	// ★ 再删整帖：三笔一次全部追回（答主 -40 / 楼主 -35）
	if rec := doWithToken(t, e.r, e.adminTok, http.MethodDelete, fmt.Sprintf("/api/admin/forum/topics/%d", top), nil); rec.Code != http.StatusOK {
		t.Fatalf("管理员删帖应 200，实际 %d %s", rec.Code, rec.Body.String())
	}
	if got := e.balance(t, e.authorTok); got != 0 {
		t.Fatalf("删帖应追回楼主 35（采纳动作 5 + 认定 30），实际 %d", got)
	}
	if got := e.balance(t, e.answererTok); got != 0 {
		t.Fatalf("删帖应追回答主 40，实际 %d", got)
	}
	rows := e.rollbackRows(t, top)
	if len(rows) != 2 {
		t.Fatalf("应恰两条回收流水（答主/楼主各一），实际 %d", len(rows))
	}
	sum := 0
	for _, rb := range rows {
		sum += int(rb.Delta)
	}
	if sum != -75 {
		t.Fatalf("回收合计应 -75（答主 -40 + 楼主 -35），实际 %d", sum)
	}

	fmt.Println("删回复/删帖键边界契约通过：先删回答不占坑，删帖仍一次追回三笔")
}

// TestForumViewCountCountsRealBrowsingOnly 浏览量只统计真实浏览：
// 排除作者自访问，且每人每日每帖只计一次（hot 排序第三键就是它，不设防即可自刷推热）。
func TestForumViewCountCountsRealBrowsingOnly(t *testing.T) {
	e := newRewardFactsEnv(t)
	top := e.createQuestion(t, e.authorTok, "浏览量防刷")

	view := func(tok string) int {
		t.Helper()
		rec := doWithToken(t, e.r, tok, http.MethodGet, fmt.Sprintf("/api/forum/topics/%d", top), nil)
		if rec.Code != http.StatusOK {
			t.Fatalf("详情应 200，实际 %d %s", rec.Code, rec.Body.String())
		}
		// 详情响应的主题在 data.topic 下（不是 data 顶层）
		var got struct {
			Data struct {
				Topic struct {
					ViewCount int `json:"view_count"`
				} `json:"topic"`
			} `json:"data"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
			t.Fatalf("解析详情失败: %v", err)
		}
		return got.Data.Topic.ViewCount
	}

	// 作者自看：不计数（旧实现每次详情请求都 +1，自刷即可推热）
	if got := view(e.authorTok); got != 0 {
		t.Fatalf("作者自看不应该计数，实际 view_count=%d", got)
	}
	if got := view(e.readerTok); got != 1 {
		t.Fatalf("读者首次浏览应计 1，实际 %d", got)
	}
	if got := view(e.readerTok); got != 1 {
		t.Fatalf("同一读者当日重复浏览不应重复计数，实际 %d", got)
	}
	if got := view(e.reader2Tok); got != 2 {
		t.Fatalf("另一位读者应各自计数到 2，实际 %d", got)
	}

	fmt.Println("浏览量契约通过：作者自看不计、同人当日不重复、不同人各自计")
}
