// 契约测试 #742 批次一：全类别精选位 + featured_bonus 幂等直记 + growth_first_experience 达成判定。
//
// 守四件事：
//  1. 精选位权限：管理端路由仅 admin 可达（hrwai_user 403）。
//  2. featured_bonus 幂等：首次加精 +30 一次；取消重精不重复发分（流水存在判定 + 占坑键）；
//     取消精选只改状态不回滚；重复加精/重复取消幂等短路。
//  3. featured=true|false 筛选与 DTO is_featured 回显（全类别可精：经验帖/讨论帖均可）。
//  4. growth_first_experience 达成判定：仅「任务上线后」的经验帖计达成（存量口径），
//     discussion/question 不计；total_limit=1 终身一次。
//     ADR-0040 起经验由管理端认定、学员发帖传 experience 已 400，经验行一律直接落库种子。
//
// 站内信（forum_featured）与积分入账同事务，经 notifications 行数断言。
package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"forklift-training/internal/config"
	"forklift-training/internal/model"
	"forklift-training/internal/security"
	"forklift-training/internal/testutil"
)

// featuredResp 加精响应（取本测试关心的字段）。
type featuredResp struct {
	Code int `json:"code"`
	Data struct {
		ID         int64  `json:"id"`
		Category   string `json:"category"`
		IsFeatured bool   `json:"is_featured"`
	} `json:"data"`
	Message string `json:"message"`
}

func TestForumFeaturedContract(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := testutil.NewMemoryDB(t)

	author := model.HrwaiUser{Account: "feat_author", Phone: "13800000103", Username: "精选作者", Status: 1, CreatedAt: testutil.Now()}
	if err := db.Create(&author).Error; err != nil {
		t.Fatalf("创建作者失败: %v", err)
	}

	cfg := &config.Config{
		JWTSecretKey: "contract-test-secret",
		AuthCookie:   config.AuthCookieConfig{Name: "hrwai_token"},
	}
	r := gin.New()
	apiGroup := r.Group("/api")
	deps := newContractDeps(t, db, cfg)
	RegisterForumRoutes(apiGroup, deps.RouterDeps(), deps.ForumSvc, deps.ForumImageSvc)
	RegisterPointsRoutes(apiGroup, deps.RouterDeps(), deps.PointsSvc)

	authorToken, err := security.NewSession(cfg.JWTSecretKey, time.Hour, security.CookieConfig{}).
		Issue(int(author.ID), author.Account, "hrwai_user")
	if err != nil {
		t.Fatalf("签发作者 token 失败: %v", err)
	}
	adminToken, err := security.NewSession(cfg.JWTSecretKey, time.Hour, security.CookieConfig{}).
		Issue(1, "admin1", "admin")
	if err != nil {
		t.Fatalf("签发 admin token 失败: %v", err)
	}

	do := func(tok, method, path string, body any) *httptest.ResponseRecorder {
		var req *http.Request
		if body != nil {
			b, _ := json.Marshal(body)
			req, _ = http.NewRequest(method, path, bytes.NewReader(b))
			req.Header.Set("Content-Type", "application/json")
		} else {
			req, _ = http.NewRequest(method, path, nil)
		}
		req.Header.Set("Authorization", "Bearer "+tok)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		return w
	}

	getBalance := func(tok string) int {
		rec := do(tok, http.MethodGet, "/api/points/balance", nil)
		if rec.Code != http.StatusOK {
			t.Fatalf("获取余额失败: %d %s", rec.Code, rec.Body.String())
		}
		var got struct {
			Code int `json:"code"`
			Data struct {
				Balance int `json:"balance"`
			} `json:"data"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
			t.Fatalf("解析余额失败: %v", err)
		}
		return got.Data.Balance
	}

	create := func(tok string, body map[string]any) featuredResp {
		t.Helper()
		rec := do(tok, http.MethodPost, "/api/forum/topics", body)
		var got featuredResp
		if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
			t.Fatalf("解析发帖响应失败: %v (body=%s)", err, rec.Body.String())
		}
		if rec.Code != http.StatusCreated {
			t.Fatalf("发帖 %v 状态码 = %d (message=%s)", body, rec.Code, got.Message)
		}
		return got
	}

	// 素材：经验帖（加精主对象）+ 讨论帖（全类别可精的对照组）。
	// ADR-0040 起学员不能自称经验，经验行直接落库模拟（存量行 / 管理端认定行）；
	// 经 API 新建的那条改为讨论帖——加精本身与类别无关，全类别可精的对照仍成立。
	preTopic := model.ForumTopic{
		Category: "experience", UserID: author.ID, Title: "备考经验精选帖", Content: "x",
		Images: model.JSONB("[]"), CreatedAt: testutil.Now(), UpdatedAt: testutil.Now(),
	}
	if err := db.Create(&preTopic).Error; err != nil {
		t.Fatalf("种子经验帖失败: %v", err)
	}
	exp := featuredResp{}
	exp.Data.ID = preTopic.ID
	disc := create(authorToken, map[string]any{"title": "普通讨论帖", "content": "x"})

	// 1. 权限：hrwai_user 走管理端加精路由必须被拒
	if rec := do(authorToken, http.MethodPost, fmt.Sprintf("/api/admin/forum/topics/%d/featured", exp.Data.ID), nil); rec.Code == http.StatusOK {
		t.Fatalf("非管理员加精应被拒，实际 %d", rec.Code)
	}

	// 2. 首次加精：状态迁移 + 帖主 +30（同事务站内信）
	if got := getBalance(authorToken); got != 0 {
		t.Fatalf("加精前余额应为 0，实际 %d", got)
	}
	rec := do(adminToken, http.MethodPost, fmt.Sprintf("/api/admin/forum/topics/%d/featured", exp.Data.ID), nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("加精应 200，实际 %d %s", rec.Code, rec.Body.String())
	}
	var feat featuredResp
	if err := json.Unmarshal(rec.Body.Bytes(), &feat); err != nil {
		t.Fatalf("解析加精响应失败: %v", err)
	}
	if !feat.Data.IsFeatured {
		t.Fatalf("加精后 is_featured 应为 true")
	}
	if got := getBalance(authorToken); got != 30 {
		t.Fatalf("首次加精后帖主余额应为 30，实际 %d", got)
	}
	var notifyCnt int64
	if err := db.Table("notifications").Where("user_id = ? AND type = ?", author.ID, "forum_featured").Count(&notifyCnt).Error; err != nil {
		t.Fatalf("查询站内信失败: %v", err)
	}
	if notifyCnt != 1 {
		t.Fatalf("加精站内信应恰好 1 条，实际 %d", notifyCnt)
	}

	// 3. 重复加精：幂等短路，余额不变
	if rec := do(adminToken, http.MethodPost, fmt.Sprintf("/api/admin/forum/topics/%d/featured", exp.Data.ID), nil); rec.Code != http.StatusOK {
		t.Fatalf("重复加精应幂等 200，实际 %d", rec.Code)
	}
	if got := getBalance(authorToken); got != 30 {
		t.Fatalf("重复加精不应重复发分，余额应仍为 30，实际 %d", got)
	}

	// 4. 取消精选：只改状态不回滚
	unfeatRec := do(adminToken, http.MethodDelete, fmt.Sprintf("/api/admin/forum/topics/%d/featured", exp.Data.ID), nil)
	if unfeatRec.Code != http.StatusOK {
		t.Fatalf("取消精选应 200，实际 %d", unfeatRec.Code)
	}
	var afterUnfeat featuredResp
	if err := json.Unmarshal(unfeatRec.Body.Bytes(), &afterUnfeat); err != nil {
		t.Fatalf("解析取消精选响应失败: %v", err)
	}
	if afterUnfeat.Data.IsFeatured {
		t.Fatalf("取消精选后 is_featured 应为 false")
	}
	if got := getBalance(authorToken); got != 30 {
		t.Fatalf("取消精选不回滚分，余额应仍为 30，实际 %d", got)
	}
	if rec := do(adminToken, http.MethodDelete, fmt.Sprintf("/api/admin/forum/topics/%d/featured", exp.Data.ID), nil); rec.Code != http.StatusOK {
		t.Fatalf("重复取消精选应幂等 200，实际 %d", rec.Code)
	}

	// 5. 取消重精：状态迁移但不再发分（流水存在判定）
	if rec := do(adminToken, http.MethodPost, fmt.Sprintf("/api/admin/forum/topics/%d/featured", exp.Data.ID), nil); rec.Code != http.StatusOK {
		t.Fatalf("取消重精应 200，实际 %d", rec.Code)
	}
	if got := getBalance(authorToken); got != 30 {
		t.Fatalf("取消重精不应重复发分，余额应仍为 30，实际 %d", got)
	}

	// 6. 全类别可精：讨论帖加精同样成立（且站内信各帖一条）
	if rec := do(adminToken, http.MethodPost, fmt.Sprintf("/api/admin/forum/topics/%d/featured", disc.Data.ID), nil); rec.Code != http.StatusOK {
		t.Fatalf("讨论帖加精应 200，实际 %d", rec.Code)
	}
	if err := db.Table("notifications").Where("user_id = ? AND type = ?", author.ID, "forum_featured").Count(&notifyCnt).Error; err != nil {
		t.Fatalf("查询站内信失败: %v", err)
	}
	if notifyCnt != 2 {
		t.Fatalf("两帖加精站内信应共 2 条，实际 %d", notifyCnt)
	}

	// 7. featured 筛选：true 只出精选、false 只出非精选、非法值 400
	list := func(query string) topicListResp {
		t.Helper()
		rec := do(authorToken, http.MethodGet, "/api/forum/topics"+query, nil)
		if rec.Code != http.StatusOK {
			t.Fatalf("列表 %s 状态码 = %d, body=%s", query, rec.Code, rec.Body.String())
		}
		var got topicListResp
		if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
			t.Fatalf("解析列表失败: %v", err)
		}
		return got
	}
	featList := list("?featured=true")
	if featList.Data.Total != 2 {
		t.Fatalf("featured=true 应出全部 2 篇精选帖，实际 total=%d", featList.Data.Total)
	}
	for _, tp := range featList.Data.Topics {
		if tp.Title != "备考经验精选帖" && tp.Title != "普通讨论帖" {
			t.Fatalf("featured=true 混入了非精选帖 %q", tp.Title)
		}
	}
	unfeatList := list("?featured=false")
	if unfeatList.Data.Total != 0 {
		t.Fatalf("featured=false 应出 0 篇（全部已被精），实际 total=%d", unfeatList.Data.Total)
	}
	if rec := do(authorToken, http.MethodGet, "/api/forum/topics?featured=bogus", nil); rec.Code != http.StatusBadRequest {
		t.Fatalf("featured 非法值应 400，实际 %d", rec.Code)
	}

	fmt.Println("精选位契约通过：权限/幂等直记/取消不回滚/全类别/featured 筛选均守住")
}

// 注：本文件原含 TestGrowthFirstExperienceContract（#742 首篇经验专项分达成判定）。
// 该任务随 ADR-0040 退役——认定奖励与加精合并为同一笔 featured_bonus，
// 任务配置行由迁移 000027 删除，points_service 的查库判定分支一并移除。
// 测试同时删除：它自行种下配置行（绕过迁移），构造的是生产上不可能存在的场景。
