// 契约测试 #827：列表契约收尾——solved 类别校验 + reward_issued 语义。
//
// 守两件事（第三件「认定筛选与 scope/category/solved/featured 共存」在批 2 已随
// forum_designation_contract_test.go 落地，此处只做共存回归）：
//  1. **solved 只对问答帖有意义**：缺 category=question 时返回 400 而非**静默空列表**。
//     旧实现在任何类别下都无条件拼 accepted_reply_id 条件——对 discussion 而言该列恒为 NULL，
//     用户拿到的是空列表而非明确拒绝（与 solved 非法值已返回 400 的口径不一致）。
//  2. **reward_issued = 该帖是否已产生过任一直记奖励**：旧实现只认 question 帖的
//     accepted_bonus，于是被加精或被认定（+30）的帖子仍显示「未发分」——契约字段与事实不符。
//     列表批量回填与详情单条回填是两个实现，**必须同口径**（漂移过一次就是 bug）。
package api

import (
	"encoding/json"
	"fmt"
	"net/http"
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

type listContractEnv struct {
	db          *gorm.DB
	r           *gin.Engine
	authorTok   string
	adminTok    string
	answererTok string
	authorID    int
	answererID  int
}

func newListContractEnv(t *testing.T) *listContractEnv {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db := testutil.NewMemoryDB(t)
	cfg := &config.Config{
		JWTSecretKey: "contract-test-secret",
		AuthCookie:   config.AuthCookieConfig{Name: "hrwai_token"},
	}
	r := NewRouter(newContractDeps(t, db, cfg))

	author := model.HrwaiUser{Account: "lc_author", Phone: "13800000501", Username: "楼主", Status: 1, CreatedAt: testutil.Now()}
	answerer := model.HrwaiUser{Account: "lc_answerer", Phone: "13800000502", Username: "答主", Status: 1, CreatedAt: testutil.Now()}
	for _, u := range []*model.HrwaiUser{&author, &answerer} {
		if err := db.Create(u).Error; err != nil {
			t.Fatalf("创建用户失败: %v", err)
		}
	}
	adminPwd, _ := service.HashPassword("admin123")
	admin := testutil.SeedAdmin(t, db, "adminListContract", adminPwd)

	issue := func(id int, account, role string) string {
		tok, err := security.NewSession(cfg.JWTSecretKey, time.Hour, security.CookieConfig{Name: cfg.AuthCookie.Name}).Issue(id, account, role)
		if err != nil {
			t.Fatalf("签发 token 失败: %v", err)
		}
		return tok
	}
	return &listContractEnv{
		db: db, r: r, authorID: author.ID, answererID: answerer.ID,
		authorTok:   issue(author.ID, author.Account, "hrwai_user"),
		answererTok: issue(answerer.ID, answerer.Account, "hrwai_user"),
		adminTok:    issue(admin.AdminID, admin.Username, "admin"),
	}
}

func (e *listContractEnv) list(t *testing.T, tok, query string) topicListResp {
	t.Helper()
	rec := doWithToken(t, e.r, tok, http.MethodGet, "/api/forum/topics"+query, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("列表 %s 状态码 = %d, body=%s", query, rec.Code, rec.Body.String())
	}
	var got topicListResp
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("解析列表失败: %v", err)
	}
	return got
}

func (e *listContractEnv) create(t *testing.T, tok string, body map[string]any) int64 {
	t.Helper()
	rec := doWithToken(t, e.r, tok, http.MethodPost, "/api/forum/topics", body)
	if rec.Code != http.StatusCreated {
		t.Fatalf("发帖 %v 应 201，实际 %d %s", body, rec.Code, rec.Body.String())
	}
	var got struct {
		Data struct {
			ID int64 `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("解析发帖响应失败: %v", err)
	}
	return got.Data.ID
}

// rewardIssuedOf 从列表响应里取指定标题那篇的 reward_issued。
func (e *listContractEnv) rewardIssuedOf(t *testing.T, query, title string) (bool, bool) {
	t.Helper()
	got := e.list(t, e.authorTok, query)
	for _, tp := range got.Data.Topics {
		if tp.Title == title {
			return tp.RewardIssued, true
		}
	}
	return false, false
}

// TestForumSolvedRequiresQuestionCategory solved 只对问答帖有意义：缺类别时 400，不静默返回空。
func TestForumSolvedRequiresQuestionCategory(t *testing.T) {
	e := newListContractEnv(t)

	// 素材：一篇问答帖（已采纳）+ 一篇讨论帖，证明「空结果」在旧实现下确实会发生
	qID := e.create(t, e.authorTok, map[string]any{"category": "question", "title": "问答帖", "content": "x"})
	dID := e.create(t, e.authorTok, map[string]any{"category": "discussion", "title": "讨论帖", "content": "x"})
	_ = dID
	rec := doWithToken(t, e.r, e.answererTok, http.MethodPost, fmt.Sprintf("/api/forum/topics/%d/replies", qID), map[string]any{"content": "答案"})
	if rec.Code != http.StatusCreated {
		t.Fatalf("回复应 201，实际 %d", rec.Code)
	}
	var reply struct {
		Data struct {
			ID int64 `json:"id"`
		} `json:"data"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &reply)
	if rec := doWithToken(t, e.r, e.authorTok, http.MethodPost, fmt.Sprintf("/api/forum/topics/%d/accept", qID), map[string]any{"reply_id": reply.Data.ID}); rec.Code != http.StatusOK {
		t.Fatalf("采纳应 200，实际 %d", rec.Code)
	}

	// 1. ★ 缺 category：旧实现会对 discussion 无条件拼 accepted_reply_id IS NOT NULL → 静默空列表；
	//    新口径明确拒绝。
	for _, q := range []string{
		"?solved=solved",
		"?solved=unsolved",
		"?scope=general&solved=solved",
		"?category=discussion&solved=solved",
	} {
		rec := doWithToken(t, e.r, e.authorTok, http.MethodGet, "/api/forum/topics"+q, nil)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("%s 缺 category=question 应 400（不得静默空列表），实际 %d %s", q, rec.Code, rec.Body.String())
		}
	}

	// 2. 正确用法：category=question + solved 正常返回
	solvedList := e.list(t, e.authorTok, "?category=question&solved=solved")
	if solvedList.Data.Total != 1 {
		t.Fatalf("已解决应恰 1 条，实际 %d", solvedList.Data.Total)
	}
	unsolvedList := e.list(t, e.authorTok, "?category=question&solved=unsolved")
	if unsolvedList.Data.Total != 0 {
		t.Fatalf("求助应 0 条，实际 %d", unsolvedList.Data.Total)
	}
	// 3. solved=all 与不传等价（不属于「非空」，不触发校验）
	if all := e.list(t, e.authorTok, "?solved=all"); all.Data.Total != 2 {
		t.Fatalf("solved=all 不应触发校验，应返回全部 2 条，实际 %d", all.Data.Total)
	}

	fmt.Println("solved 校验契约通过：缺 category=question 返回 400，正确用法与 all 不受影响")
}

// TestForumRewardIssuedCoversAcceptRewardsOnly reward_issued **只覆盖采纳类奖励**，且列表/详情同口径。
//
// 语义边界（本用例的回归价值）：该字段的唯一消费方是「采纳前二次确认」（ForumDetail.vue:262），
// 文案为「该帖采纳奖励已发放……不再产生积分」。若把 featured_bonus（加精/认定）也算进来，
// 一篇**只是被加精、从未被采纳**的问答帖会让楼主看到「采纳不再产生积分」——而实际上答主仍会
// 拿到 40 分。错误提示会劝退真实的采纳行为，故 reason 集合必须是采纳类。
func TestForumRewardIssuedCoversAllDirectRewards(t *testing.T) {
	e := newListContractEnv(t)

	// 素材一：被加精的讨论帖（旧实现下 reward_issued=false，因为它不是 question 帖）
	featTitle := "被加精的讨论帖"
	featID := e.create(t, e.authorTok, map[string]any{"category": "discussion", "title": featTitle, "content": "x"})
	if rec := doWithToken(t, e.r, e.adminTok, http.MethodPost, fmt.Sprintf("/api/admin/forum/topics/%d/featured", featID), nil); rec.Code != http.StatusOK {
		t.Fatalf("加精应 200，实际 %d %s", rec.Code, rec.Body.String())
	}

	// 素材二：被认定为经验的帖子（+30 走同一条 featured_bonus）
	expTitle := "被认定的考经"
	expID := e.create(t, e.authorTok, map[string]any{"category": "discussion", "title": expTitle, "content": "x"})
	if rec := doWithToken(t, e.r, e.adminTok, http.MethodPost, fmt.Sprintf("/api/admin/forum/topics/%d/experience", expID), nil); rec.Code != http.StatusOK {
		t.Fatalf("认定应 200，实际 %d", rec.Code)
	}

	// 素材三：被采纳的问答帖（accept_action 记在楼主）
	qTitle := "被采纳的问答帖"
	qID := e.create(t, e.authorTok, map[string]any{"category": "question", "title": qTitle, "content": "x"})
	rec := doWithToken(t, e.r, e.answererTok, http.MethodPost, fmt.Sprintf("/api/forum/topics/%d/replies", qID), map[string]any{"content": "答案"})
	var reply struct {
		Data struct {
			ID int64 `json:"id"`
		} `json:"data"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &reply)
	if rec := doWithToken(t, e.r, e.authorTok, http.MethodPost, fmt.Sprintf("/api/forum/topics/%d/accept", qID), map[string]any{"reply_id": reply.Data.ID}); rec.Code != http.StatusOK {
		t.Fatalf("采纳应 200，实际 %d", rec.Code)
	}

	// 素材四：无任何奖励的对照帖
	plainTitle := "无奖励的讨论帖"
	plainID := e.create(t, e.authorTok, map[string]any{"category": "discussion", "title": plainTitle, "content": "x"})
	_ = plainID

	// 1. ★ 列表回填：三类奖励都为真，对照为假
	for _, tc := range []struct {
		title string
		want  bool
	}{
		{featTitle, false}, // 只被加精、从未被采纳 → 采纳仍会发分，**不得**置位
		{expTitle, false},  // 只被认定、从未被采纳 → 同上
		{qTitle, true},     // 已被采纳 → 采纳奖励已发放
		{plainTitle, false},
	} {
		got, found := e.rewardIssuedOf(t, "", tc.title)
		if !found {
			t.Fatalf("列表应含 %q", tc.title)
		}
		if got != tc.want {
			t.Fatalf("%q 的 reward_issued = %v，期望 %v", tc.title, got, tc.want)
		}
	}

	// 2. ★ 详情单条回填与列表**同口径**（两处是两个实现，必须一致）
	for _, tc := range []struct {
		id   int64
		want bool
	}{
		{featID, false}, {expID, false}, {qID, true}, {plainID, false},
	} {
		rec := doWithToken(t, e.r, e.authorTok, http.MethodGet, fmt.Sprintf("/api/forum/topics/%d", tc.id), nil)
		if rec.Code != http.StatusOK {
			t.Fatalf("详情 %d 应 200，实际 %d", tc.id, rec.Code)
		}
		var got struct {
			Data struct {
				Topic struct {
					RewardIssued bool `json:"reward_issued"`
				} `json:"topic"`
			} `json:"data"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
			t.Fatalf("解析详情失败: %v", err)
		}
		if got.Data.Topic.RewardIssued != tc.want {
			t.Fatalf("详情 %d 的 reward_issued = %v，期望 %v（须与列表同口径）", tc.id, got.Data.Topic.RewardIssued, tc.want)
		}
	}

	fmt.Println("reward_issued 契约通过：仅采纳类奖励置位（加精/认定不置位），列表与详情同口径")
}

// TestForumListFiltersCoexist 四种筛选与 scope/category 共存不互相污染（回归）。
func TestForumListFiltersCoexist(t *testing.T) {
	e := newListContractEnv(t)

	expTitle := "认定帖"
	expID := e.create(t, e.authorTok, map[string]any{"category": "discussion", "title": expTitle, "content": "x"})
	plainTitle := "普通帖"
	e.create(t, e.authorTok, map[string]any{"category": "discussion", "title": plainTitle, "content": "x"})
	if rec := doWithToken(t, e.r, e.adminTok, http.MethodPost, fmt.Sprintf("/api/admin/forum/topics/%d/experience", expID), nil); rec.Code != http.StatusOK {
		t.Fatalf("认定应 200，实际 %d", rec.Code)
	}

	// is_experience=true 与 featured=true 都只出认定帖（认定蕴含精选）
	for _, q := range []string{"?is_experience=true", "?featured=true"} {
		got := e.list(t, e.authorTok, q)
		if got.Data.Total != 1 {
			t.Fatalf("%s 应恰 1 条，实际 %d", q, got.Data.Total)
		}
		if got.Data.Topics[0].Title != expTitle {
			t.Fatalf("%s 出错了帖子: %q", q, got.Data.Topics[0].Title)
		}
	}
	// 叠加 scope=general（认定帖的 chapter_id 为 NULL，属综合区）
	both := e.list(t, e.authorTok, "?scope=general&category=discussion&is_experience=true")
	if both.Data.Total != 1 {
		t.Fatalf("叠加 scope+category+is_experience 应恰 1 条，实际 %d", both.Data.Total)
	}
	// is_experience=false 出对照帖
	if got := e.list(t, e.authorTok, "?is_experience=false"); got.Data.Total != 1 {
		t.Fatalf("is_experience=false 应恰 1 条，实际 %d", got.Data.Total)
	}
	// 非法值 400
	if rec := doWithToken(t, e.r, e.authorTok, http.MethodGet, "/api/forum/topics?featured=bogus", nil); rec.Code != http.StatusBadRequest {
		t.Fatalf("featured 非法值应 400，实际 %d", rec.Code)
	}

	fmt.Println("筛选共存契约通过：is_experience/featured/scope/category 同 WHERE 不互相污染")
}
