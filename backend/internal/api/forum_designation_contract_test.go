// 契约测试 ADR-0040：论坛「意图 / 认定」分离的写入、认定与筛选语义。
//
// ADR-0040 把两义拆开：category 只表达学员自述的意图（discussion | question），
// 「备考经验」改为管理端认定（forum_topics.is_experience，蕴含 is_featured）。
// 本文件守六件事：
//  1. **学员不能自称经验**：发帖/编辑传 category=experience 一律 400（写入路径已收窄）。
//  2. **认定是一个动作**：认定经验同时置 is_experience 与 is_featured，并给帖主一次性 +30。
//  3. **认定奖励每帖一次**：重复认定、先加精后认定、先认定后加精，都只发一笔
//     （与加精共用同一条 featured_bonus 流水）。
//  4. **取消认定保留精选**：撤的是归类不是质量认可；已发分不回滚。
//  5. **经验帖不能直接撤精**：蕴含式由 service 拒绝并给出逃生口（库层另有 CHECK 兜底）。
//  6. **筛选各归其位**：is_experience=true 只出认定行；**经验帖仍留在讨论区**
//     （认定是叠加标记，与 is_featured 同构——加精不搬走帖子，认定也不搬走），
//     且问答 Tab 不得混入（它的意图是 discussion，本就不该进问答）。
//
// 说明：SQLite 测试库走 AutoMigrate 建表、不执行 migrations/，故库层 CHECK
// （chk_forum_topics_experience_requires_featured 等）不在本文件覆盖范围；
// 它们的守护在 forum_experience_migration_contract_test.go（Postgres adapter）。
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

// designationEnv 本文件共用装配：整套路由 + 作者/管理员两个身份。
type designationEnv struct {
	db        *gorm.DB
	r         *gin.Engine
	author    model.HrwaiUser
	reader    model.HrwaiUser
	authorTok string
	readerTok string
	adminTok  string
}

func newDesignationEnv(t *testing.T) *designationEnv {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db := testutil.NewMemoryDB(t)
	cfg := &config.Config{
		JWTSecretKey: "contract-test-secret",
		AuthCookie:   config.AuthCookieConfig{Name: "hrwai_token"},
	}
	r := NewRouter(newContractDeps(t, db, cfg))

	author := model.HrwaiUser{Account: "desig_author", Phone: "13800000401", Username: "经验作者", Status: 1, CreatedAt: testutil.Now()}
	reader := model.HrwaiUser{Account: "desig_reader", Phone: "13800000402", Username: "读者", Status: 1, CreatedAt: testutil.Now()}
	for _, u := range []*model.HrwaiUser{&author, &reader} {
		if err := db.Create(u).Error; err != nil {
			t.Fatalf("创建用户失败: %v", err)
		}
	}
	adminPwd, _ := service.HashPassword("admin123")
	admin := testutil.SeedAdmin(t, db, "adminDesig", adminPwd)

	issue := func(id int, account, role string) string {
		tok, err := security.NewSession(cfg.JWTSecretKey, time.Hour, security.CookieConfig{Name: cfg.AuthCookie.Name}).Issue(id, account, role)
		if err != nil {
			t.Fatalf("签发 token 失败: %v", err)
		}
		return tok
	}
	return &designationEnv{
		db: db, r: r, author: author, reader: reader,
		authorTok: issue(author.ID, author.Account, "hrwai_user"),
		readerTok: issue(reader.ID, reader.Account, "hrwai_user"),
		adminTok:  issue(admin.AdminID, admin.Username, "admin"),
	}
}

func (e *designationEnv) balance(t *testing.T, tok string) int {
	t.Helper()
	rec := doWithToken(t, e.r, tok, http.MethodGet, "/api/points/balance", nil)
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

func (e *designationEnv) createDiscussion(t *testing.T, title string) int64 {
	t.Helper()
	rec := doWithToken(t, e.r, e.authorTok, http.MethodPost, "/api/forum/topics",
		map[string]any{"category": "discussion", "title": title, "content": "x"})
	if rec.Code != http.StatusCreated {
		t.Fatalf("发帖应 201，实际 %d %s", rec.Code, rec.Body.String())
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

// topicFlags 回读主题快照（两轴 + 标题），断言只认落库事实、不经响应自证。
func (e *designationEnv) topicFlags(t *testing.T, id int64) (category string, featured, experience bool) {
	t.Helper()
	var row model.ForumTopic
	if err := e.db.First(&row, id).Error; err != nil {
		t.Fatalf("回读主题失败: %v", err)
	}
	return row.Category, row.IsFeatured, row.IsExperience
}

func (e *designationEnv) featuredLedgerCount(t *testing.T, topicID int64) int64 {
	t.Helper()
	var n int64
	if err := e.db.Model(&model.PointsLedger{}).
		Where("ref_type = ? AND ref_id = ? AND reason = ?", "forum_topic", fmt.Sprintf("%d", topicID), "featured_bonus").
		Count(&n).Error; err != nil {
		t.Fatalf("查询认定流水失败: %v", err)
	}
	return n
}

// TestForumIntentNarrowedToTwoValues 学员不能自称「备考经验」——写入路径已收窄。
func TestForumIntentNarrowedToTwoValues(t *testing.T) {
	e := newDesignationEnv(t)

	// 1. 发帖传 experience → 400（旧实现把它当第三类别放行，本条即该收窄的守卫）
	rec := doWithToken(t, e.r, e.authorTok, http.MethodPost, "/api/forum/topics",
		map[string]any{"category": "experience", "title": "叉车证备考心得", "content": "一次性通过"})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("发帖传 experience 应 400，实际 %d %s", rec.Code, rec.Body.String())
	}

	// 2. 挂章节的 experience 同样被拒：拒绝发生在意图值域，与章节归属无关
	rec = doWithToken(t, e.r, e.authorTok, http.MethodPost, "/api/forum/topics",
		map[string]any{"category": "experience", "chapter_id": 78, "title": "章节内备考笔记", "content": "x"})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("挂章节的 experience 也应 400，实际 %d %s", rec.Code, rec.Body.String())
	}

	// 3. 编辑同样收窄
	topicID := e.createDiscussion(t, "普通讨论帖")
	rec = doWithToken(t, e.r, e.authorTok, http.MethodPut, fmt.Sprintf("/api/forum/topics/%d", topicID),
		map[string]any{"title": "想改成经验", "content": "x", "category": "experience"})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("编辑改 experience 应 400，实际 %d %s", rec.Code, rec.Body.String())
	}

	// 4. 两值仍可自由迁移（收窄不等于冻结）
	rec = doWithToken(t, e.r, e.authorTok, http.MethodPut, fmt.Sprintf("/api/forum/topics/%d", topicID),
		map[string]any{"title": "改成问答", "content": "x", "category": "question"})
	if rec.Code != http.StatusOK {
		t.Fatalf("意图两值间迁移应 200，实际 %d %s", rec.Code, rec.Body.String())
	}

	fmt.Println("意图收窄契约通过：发帖/编辑传 experience 均 400，两值间仍可迁移")
}

// TestForumDesignateExperienceIsOneAction 认定是一个动作：两个标记同时置位 + 一次性 +30 + 站内信。
func TestForumDesignateExperienceIsOneAction(t *testing.T) {
	e := newDesignationEnv(t)
	topicID := e.createDiscussion(t, "值得推荐的备考经验")

	if got := e.balance(t, e.authorTok); got != 0 {
		t.Fatalf("认定前余额应为 0，实际 %d", got)
	}

	rec := doWithToken(t, e.r, e.adminTok, http.MethodPost, fmt.Sprintf("/api/admin/forum/topics/%d/experience", topicID), nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("认定经验应 200，实际 %d %s", rec.Code, rec.Body.String())
	}

	// 两轴同时置位（经验蕴含精选），且意图不变（category 仍是学员自述的 discussion）
	category, featured, experience := e.topicFlags(t, topicID)
	if !experience || !featured {
		t.Fatalf("认定应同时置 is_experience 与 is_featured，实际 experience=%v featured=%v", experience, featured)
	}
	if category != "discussion" {
		t.Fatalf("认定不应改意图，category 应仍为 discussion，实际 %q", category)
	}
	if got := e.balance(t, e.authorTok); got != 30 {
		t.Fatalf("首次认定应 +30，实际 %d", got)
	}
	// 站内信与到账同事务
	var notifyCnt int64
	if err := e.db.Table("notifications").Where("user_id = ? AND type = ?", e.author.ID, "forum_featured").Count(&notifyCnt).Error; err != nil {
		t.Fatalf("查询站内信失败: %v", err)
	}
	if notifyCnt != 1 {
		t.Fatalf("认定应产生 1 条站内信，实际 %d", notifyCnt)
	}

	fmt.Println("认定动作契约通过：两标记同置、意图不变、+30 与站内信同事务")
}

// TestForumDesignationRewardIsOncePerTopic 认定奖励与加精共用一笔，三种顺序都只发一次。
func TestForumDesignationRewardIsOncePerTopic(t *testing.T) {
	e := newDesignationEnv(t)

	post := func(path string) *httptest.ResponseRecorder {
		return doWithToken(t, e.r, e.adminTok, http.MethodPost, path, nil)
	}

	// 场景 A：重复认定
	topicA := e.createDiscussion(t, "重复认定")
	if rec := post(fmt.Sprintf("/api/admin/forum/topics/%d/experience", topicA)); rec.Code != http.StatusOK {
		t.Fatalf("首次认定应 200，实际 %d", rec.Code)
	}
	if rec := post(fmt.Sprintf("/api/admin/forum/topics/%d/experience", topicA)); rec.Code != http.StatusOK {
		t.Fatalf("重复认定应幂等 200，实际 %d %s", rec.Code, rec.Body.String())
	}
	if n := e.featuredLedgerCount(t, topicA); n != 1 {
		t.Fatalf("重复认定不得重复发分，流水应 1 条，实际 %d", n)
	}

	// 场景 B：先加精，后认定 —— 认定不再发第二笔
	topicB := e.createDiscussion(t, "先精后认定")
	if rec := post(fmt.Sprintf("/api/admin/forum/topics/%d/featured", topicB)); rec.Code != http.StatusOK {
		t.Fatalf("加精应 200，实际 %d", rec.Code)
	}
	beforeB := e.balance(t, e.authorTok)
	if rec := post(fmt.Sprintf("/api/admin/forum/topics/%d/experience", topicB)); rec.Code != http.StatusOK {
		t.Fatalf("加精后认定应 200，实际 %d %s", rec.Code, rec.Body.String())
	}
	if delta := e.balance(t, e.authorTok) - beforeB; delta != 0 {
		t.Fatalf("先加精后认定不得重复发分，实际 +%d", delta)
	}
	if _, featured, experience := e.topicFlags(t, topicB); !featured || !experience {
		t.Fatalf("先加精后认定应两轴皆真，实际 featured=%v experience=%v", featured, experience)
	}
	if n := e.featuredLedgerCount(t, topicB); n != 1 {
		t.Fatalf("先精后认定流水应 1 条，实际 %d", n)
	}

	// 场景 C：先认定，后加精（加精路径的流水存在判定应短路）
	topicC := e.createDiscussion(t, "先认定后精")
	if rec := post(fmt.Sprintf("/api/admin/forum/topics/%d/experience", topicC)); rec.Code != http.StatusOK {
		t.Fatalf("认定应 200，实际 %d", rec.Code)
	}
	beforeC := e.balance(t, e.authorTok)
	// 加精时 is_featured 已是 true，走幂等短路；无论走哪条都不该再发分
	if rec := post(fmt.Sprintf("/api/admin/forum/topics/%d/featured", topicC)); rec.Code != http.StatusOK {
		t.Fatalf("已精选帖再加精应幂等 200，实际 %d %s", rec.Code, rec.Body.String())
	}
	if delta := e.balance(t, e.authorTok) - beforeC; delta != 0 {
		t.Fatalf("先认定后加精不得重复发分，实际 +%d", delta)
	}
	if n := e.featuredLedgerCount(t, topicC); n != 1 {
		t.Fatalf("先认定后精流水应 1 条，实际 %d", n)
	}

	fmt.Println("认定奖励幂等契约通过：重复认定/先精后认定/先认定后精均只发一笔")
}

// TestForumRevokeExperienceKeepsFeatured 取消认定只撤归类，保留精选位且不回滚已发分。
func TestForumRevokeExperienceKeepsFeatured(t *testing.T) {
	e := newDesignationEnv(t)
	topicID := e.createDiscussion(t, "认定后取消")
	if rec := doWithToken(t, e.r, e.adminTok, http.MethodPost, fmt.Sprintf("/api/admin/forum/topics/%d/experience", topicID), nil); rec.Code != http.StatusOK {
		t.Fatalf("认定应 200，实际 %d", rec.Code)
	}
	balanceAfterDesignate := e.balance(t, e.authorTok)

	rec := doWithToken(t, e.r, e.adminTok, http.MethodDelete, fmt.Sprintf("/api/admin/forum/topics/%d/experience", topicID), nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("取消认定应 200，实际 %d %s", rec.Code, rec.Body.String())
	}
	_, featured, experience := e.topicFlags(t, topicID)
	if experience {
		t.Fatalf("取消认定后 is_experience 应为 false")
	}
	if !featured {
		t.Fatalf("取消认定应保留精选位（撤的是归类不是质量认可），实际 is_featured=false")
	}
	if got := e.balance(t, e.authorTok); got != balanceAfterDesignate {
		t.Fatalf("取消认定不应回滚已发分，期望 %d 实际 %d", balanceAfterDesignate, got)
	}

	// 幂等：重复取消
	if rec := doWithToken(t, e.r, e.adminTok, http.MethodDelete, fmt.Sprintf("/api/admin/forum/topics/%d/experience", topicID), nil); rec.Code != http.StatusOK {
		t.Fatalf("重复取消认定应幂等 200，实际 %d", rec.Code)
	}

	fmt.Println("取消认定契约通过：保留精选、不回滚已发分、重复取消幂等")
}

// TestForumUnfeatureBlockedOnExperienceTopic 经验蕴含精选：经验帖不能直接撤精，须先取消认定。
func TestForumUnfeatureBlockedOnExperienceTopic(t *testing.T) {
	e := newDesignationEnv(t)
	topicID := e.createDiscussion(t, "经验帖撤精")
	if rec := doWithToken(t, e.r, e.adminTok, http.MethodPost, fmt.Sprintf("/api/admin/forum/topics/%d/experience", topicID), nil); rec.Code != http.StatusOK {
		t.Fatalf("认定应 200，实际 %d", rec.Code)
	}

	// 直接撤精 → 400 + 逃生口文案（前端据此提示「先取消经验认定」）
	rec := doWithToken(t, e.r, e.adminTok, http.MethodDelete, fmt.Sprintf("/api/admin/forum/topics/%d/featured", topicID), nil)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("经验帖撤精应 400，实际 %d %s", rec.Code, rec.Body.String())
	}
	var got struct {
		Message string `json:"message"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &got)
	if got.Message != "备考经验帖蕴含精选位，请先取消经验认定" {
		t.Fatalf("拒绝文案不符，实际 %q", got.Message)
	}
	if _, featured, experience := e.topicFlags(t, topicID); !featured || !experience {
		t.Fatalf("被拒的撤精不得改动状态，实际 featured=%v experience=%v", featured, experience)
	}

	// 逃生口：先取消认定，再撤精即放行
	if rec := doWithToken(t, e.r, e.adminTok, http.MethodDelete, fmt.Sprintf("/api/admin/forum/topics/%d/experience", topicID), nil); rec.Code != http.StatusOK {
		t.Fatalf("取消认定应 200，实际 %d", rec.Code)
	}
	if rec := doWithToken(t, e.r, e.adminTok, http.MethodDelete, fmt.Sprintf("/api/admin/forum/topics/%d/featured", topicID), nil); rec.Code != http.StatusOK {
		t.Fatalf("取消认定后撤精应 200，实际 %d %s", rec.Code, rec.Body.String())
	}
	if _, featured, _ := e.topicFlags(t, topicID); featured {
		t.Fatalf("撤精后 is_featured 应为 false")
	}

	fmt.Println("蕴含式契约通过：经验帖撤精被拒并给逃生口，先取消认定后放行")
}

// TestForumExperienceFilterDoesNotLeak 认定筛选与意图/区域筛选共存不串区。
func TestForumExperienceFilterDoesNotLeak(t *testing.T) {
	e := newDesignationEnv(t)

	expTopic := e.createDiscussion(t, "被认定的考经")
	discTopic := e.createDiscussion(t, "普通讨论帖")
	if rec := doWithToken(t, e.r, e.authorTok, http.MethodPost, "/api/forum/topics",
		map[string]any{"category": "question", "title": "问答帖", "content": "x"}); rec.Code != http.StatusCreated {
		t.Fatalf("发问答帖应 201，实际 %d", rec.Code)
	}
	if rec := doWithToken(t, e.r, e.adminTok, http.MethodPost, fmt.Sprintf("/api/admin/forum/topics/%d/experience", expTopic), nil); rec.Code != http.StatusOK {
		t.Fatalf("认定应 200，实际 %d", rec.Code)
	}

	list := func(query string) topicListResp {
		t.Helper()
		rec := doWithToken(t, e.r, e.readerTok, http.MethodGet, "/api/forum/topics"+query, nil)
		if rec.Code != http.StatusOK {
			t.Fatalf("列表 %s 状态码 = %d, body=%s", query, rec.Code, rec.Body.String())
		}
		var got topicListResp
		if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
			t.Fatalf("解析列表失败: %v", err)
		}
		return got
	}

	// 1. is_experience=true 恰出认定行
	expTab := list("?is_experience=true")
	if expTab.Data.Total != 1 {
		t.Fatalf("经验 Tab 应恰 1 条，实际 %d", expTab.Data.Total)
	}
	if expTab.Data.Topics[0].Title != "被认定的考经" {
		t.Fatalf("经验 Tab 出错了帖子: %q", expTab.Data.Topics[0].Title)
	}

	// 2. ★ 经验帖**留在**讨论 Tab（ADR-0040：认定是叠加标记，与 is_featured 同构——
	//    加精不搬走帖子，认定也不搬走；经验 Tab 只是它的子集视图）。
	//    注意这与改造前不同：旧 category='experience' 天然把它挡在讨论区外。
	discTab := list("?scope=general&category=discussion")
	if got := discTab.titles(); !got["被认定的考经"] {
		t.Fatalf("经验帖应留在讨论 Tab（认定不搬走帖子），实际 %v", discTab.Data.Topics)
	} else if !got["普通讨论帖"] {
		t.Fatalf("讨论 Tab 应含讨论帖，实际 %v", discTab.Data.Topics)
	}

	// 3. 问答 Tab 不得混入（经验帖的意图是 discussion，本就不该进问答）
	if got := list("?category=question").titles(); got["被认定的考经"] {
		t.Fatalf("经验帖灌进了问答 Tab")
	}

	// 4. 非法值 400
	if rec := doWithToken(t, e.r, e.readerTok, http.MethodGet, "/api/forum/topics?is_experience=bogus", nil); rec.Code != http.StatusBadRequest {
		t.Fatalf("is_experience 非法值应 400，实际 %d", rec.Code)
	}

	// 5. 取消认定只撤标记：从经验 Tab 消失，但本就在讨论区、不因取消而搬走
	if rec := doWithToken(t, e.r, e.adminTok, http.MethodDelete, fmt.Sprintf("/api/admin/forum/topics/%d/experience", expTopic), nil); rec.Code != http.StatusOK {
		t.Fatalf("取消认定应 200，实际 %d", rec.Code)
	}
	if got := list("?is_experience=true").Data.Total; got != 0 {
		t.Fatalf("取消认定后经验 Tab 应空，实际 %d", got)
	}
	if got := list("?scope=general&category=discussion").titles(); !got["被认定的考经"] {
		t.Fatalf("取消认定后帖子应仍在讨论区（反向锁：不因撤销而搬走），实际 %v", got)
	}

	_ = discTopic
	fmt.Println("认定筛选契约通过：经验帖留在讨论区、不串问答 Tab、非法值 400、取消认定只撤标记")
}

// TestForumDesignationRequiresAdmin 认定端点仅管理员可达。
func TestForumDesignationRequiresAdmin(t *testing.T) {
	e := newDesignationEnv(t)
	topicID := e.createDiscussion(t, "权限校验")

	for _, tc := range []struct {
		method string
		path   string
	}{
		{http.MethodPost, fmt.Sprintf("/api/admin/forum/topics/%d/experience", topicID)},
		{http.MethodDelete, fmt.Sprintf("/api/admin/forum/topics/%d/experience", topicID)},
	} {
		// 学员 token → 403
		if rec := doWithToken(t, e.r, e.authorTok, tc.method, tc.path, nil); rec.Code != http.StatusForbidden {
			t.Fatalf("%s %s 学员应 403，实际 %d", tc.method, tc.path, rec.Code)
		}
		// 未认证 → 401
		req, _ := http.NewRequest(tc.method, tc.path, nil)
		plain := httptest.NewRecorder()
		e.r.ServeHTTP(plain, req)
		if plain.Code != http.StatusUnauthorized {
			t.Fatalf("%s %s 未认证应 401，实际 %d", tc.method, tc.path, plain.Code)
		}
	}
	// 权限被拒不得留下任何认定痕迹
	if _, featured, experience := e.topicFlags(t, topicID); featured || experience {
		t.Fatalf("被拒的请求不得改动状态，实际 featured=%v experience=%v", featured, experience)
	}

	// 主题不存在 → 400（ErrTopicNotFound）
	if rec := doWithToken(t, e.r, e.adminTok, http.MethodPost, "/api/admin/forum/topics/999999/experience", nil); rec.Code != http.StatusBadRequest {
		t.Fatalf("不存在主题认定应 400，实际 %d %s", rec.Code, rec.Body.String())
	}

	fmt.Println("认定权限契约通过：学员 403 / 未认证 401 / 不存在 400，且不改状态")
}

// TestForumExperienceTopicNotAcceptable 经验帖不可被采纳（采纳之门只看意图 question）。
func TestForumExperienceTopicNotAcceptable(t *testing.T) {
	e := newDesignationEnv(t)
	topicID := e.createDiscussion(t, "被认定的考经不可采纳")
	if rec := doWithToken(t, e.r, e.adminTok, http.MethodPost, fmt.Sprintf("/api/admin/forum/topics/%d/experience", topicID), nil); rec.Code != http.StatusOK {
		t.Fatalf("认定应 200，实际 %d", rec.Code)
	}
	// 认定不改意图，故采纳之门仍在 category 上：discussion 帖不可采纳（reply_id 传 999 即可触发意图校验）
	rec := doWithToken(t, e.r, e.authorTok, http.MethodPost, fmt.Sprintf("/api/forum/topics/%d/accept", topicID), map[string]any{"reply_id": 999})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("经验帖采纳应 400，实际 %d %s", rec.Code, rec.Body.String())
	}
	fmt.Println("采纳闸门契约通过：被认定为经验的帖子仍不可被采纳")
}
