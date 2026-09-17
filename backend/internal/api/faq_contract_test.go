// #1079 帮助中心契约测试（主 seam：HTTP contract，router → httptest → 断言状态码与响应形状）。
//
// 覆盖不变式：
//   - 学员面只返回 **enabled 分类 + published 条目**；停用分类与未发布条目对学员不可见；
//   - 管理面需要 faq.manage：未认证 401、学员 403；
//   - 分类标识唯一（重复 → 409）；校验类错误 → 400；
//   - 删分类**连带删除**其下条目（外键 ON DELETE CASCADE）。
package api

import (
	"encoding/json"
	"net/http"
	"strings"
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

// seedFaqFixture 播三类内容：启用分类（2 已发布 + 1 未发布）、停用分类（1 已发布）。
// 返回启用分类与停用分类的 id。
func seedFaqFixture(t *testing.T, db *gorm.DB) (liveID, hiddenID int) {
	t.Helper()
	now := testutil.Now()
	live := model.FaqCategory{Code: "account", Title: "账号与登录", SortOrder: 1, Enabled: true, CreatedAt: now, UpdatedAt: now}
	hidden := model.FaqCategory{Code: "legacy", Title: "已停用分类", SortOrder: 2, Enabled: false, CreatedAt: now, UpdatedAt: now}
	for _, c := range []*model.FaqCategory{&live, &hidden} {
		if err := db.Create(c).Error; err != nil {
			t.Fatalf("建分类失败: %v", err)
		}
	}
	entries := []model.Faq{
		{CategoryID: live.ID, Question: "怎么注册？", Answer: "用手机号或邮箱验证码注册。", SortOrder: 1, Published: true, CreatedAt: now, UpdatedAt: now},
		{CategoryID: live.ID, Question: "忘记密码怎么办？", Answer: "走忘记密码找回。", SortOrder: 2, Published: true, CreatedAt: now, UpdatedAt: now},
		{CategoryID: live.ID, Question: "草稿条目", Answer: "未发布，学员不该看到。", SortOrder: 3, Published: false, CreatedAt: now, UpdatedAt: now},
		{CategoryID: hidden.ID, Question: "停用分类下的条目", Answer: "分类停用后整段不出现。", SortOrder: 1, Published: true, CreatedAt: now, UpdatedAt: now},
	}
	for i := range entries {
		if err := db.Create(&entries[i]).Error; err != nil {
			t.Fatalf("建条目失败: %v", err)
		}
	}
	return live.ID, hidden.ID
}

func TestFaqContract(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := testutil.NewMemoryDB(t)
	liveID, hiddenID := seedFaqFixture(t, db)

	cfg := &config.Config{JWTSecretKey: "faq-contract-secret", JWTExpiresHours: 2, JWTRefreshExpiresDays: 7}
	deps := newContractDeps(t, db, cfg)
	r := NewRouter(deps)

	pwd, _ := service.HashPassword("student123")
	student := testutil.SeedStudent(t, db, "faq_stu", pwd)
	admin := testutil.SeedAdmin(t, db, "faq_admin", pwd)
	sess := security.NewSession(cfg.JWTSecretKey, time.Hour, security.CookieConfig{})
	stuToken, err := sess.Issue(int(student.ID), student.Username, "hrwai_user")
	if err != nil {
		t.Fatalf("签发学员 token 失败: %v", err)
	}
	adminToken, err := sess.Issue(admin.AdminID, admin.Username, "admin")
	if err != nil {
		t.Fatalf("签发管理员 token 失败: %v", err)
	}

	// ===== 1. 未认证：学员面 401 =====
	if rec := performRequest(r, http.MethodGet, "/api/faq"); rec.Code != http.StatusUnauthorized {
		t.Fatalf("未认证访问 /api/faq 应 401, got %d", rec.Code)
	}

	// ===== 2. 学员面：只含 enabled 分类 + published 条目 =====
	rec := doWithToken(t, r, stuToken, http.MethodGet, "/api/faq", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("学员访问 /api/faq 应 200, got %d %s", rec.Code, rec.Body.String())
	}
	var studentBody struct {
		Data struct {
			Categories []struct {
				Code    string `json:"code"`
				ID      int    `json:"id"`
				Entries []struct {
					Question string `json:"question"`
				} `json:"entries"`
			} `json:"categories"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &studentBody); err != nil {
		t.Fatalf("解析响应失败: %v", err)
	}
	cats := studentBody.Data.Categories
	if len(cats) != 1 || cats[0].Code != "account" {
		t.Fatalf("学员面应只见 1 个启用分类 account, got %+v", cats)
	}
	if cats[0].ID != liveID {
		t.Fatalf("分类 id 应为 %d, got %d", liveID, cats[0].ID)
	}
	if len(cats[0].Entries) != 2 {
		t.Fatalf("学员面应只见 2 条已发布条目（未发布那条不得出现）, got %d", len(cats[0].Entries))
	}
	for _, e := range cats[0].Entries {
		if e.Question == "草稿条目" {
			t.Fatal("未发布条目泄漏到学员面")
		}
	}
	if cats[0].Code == "legacy" {
		t.Fatal("停用分类泄漏到学员面")
	}

	// ===== 3. 管理面鉴权：学员 403 =====
	if rec := doWithToken(t, r, stuToken, http.MethodGet, "/api/admin/faq/categories", nil); rec.Code != http.StatusForbidden {
		t.Fatalf("学员访问管理面应 403, got %d", rec.Code)
	}
	if rec := performRequest(r, http.MethodGet, "/api/admin/faq/categories"); rec.Code != http.StatusUnauthorized {
		t.Fatalf("未认证访问管理面应 401, got %d", rec.Code)
	}

	// ===== 4. 管理面清单：含停用分类与未发布条目，带计数 =====
	rec = doWithToken(t, r, adminToken, http.MethodGet, "/api/admin/faq/categories", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("管理员列分类应 200, got %d %s", rec.Code, rec.Body.String())
	}
	var adminBody struct {
		Data struct {
			Categories []struct {
				Code       string `json:"code"`
				Enabled    bool   `json:"enabled"`
				EntryCount int64  `json:"entry_count"`
			} `json:"categories"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &adminBody); err != nil {
		t.Fatalf("解析管理面响应失败: %v", err)
	}
	if len(adminBody.Data.Categories) != 2 {
		t.Fatalf("管理面应见 2 个分类（含停用）, got %d", len(adminBody.Data.Categories))
	}
	for _, c := range adminBody.Data.Categories {
		if c.Code == "account" && c.EntryCount != 3 {
			t.Fatalf("account 计数应为 3（含未发布）, got %d", c.EntryCount)
		}
		if c.Code == "legacy" && c.Enabled {
			t.Fatal("legacy 应标记为停用")
		}
	}

	// ===== 5. 分类标识唯一：重复 → 409 =====
	rec = doWithToken(t, r, adminToken, http.MethodPost, "/api/admin/faq/categories", map[string]any{
		"code": "account", "title": "重复标识", "sort_order": 9, "enabled": true,
	})
	// 标识冲突按 400 表达（本仓未使用 409；见 faq.go 里 faqErrStatus 的注释）
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "分类标识已存在") {
		t.Fatalf("重复分类标识应 400「分类标识已存在」, got %d %s", rec.Code, rec.Body.String())
	}

	// ===== 6. 校验类错误 → 400（空问题 / 非法标识） =====
	rec = doWithToken(t, r, adminToken, http.MethodPost, "/api/admin/faq/categories", map[string]any{
		"code": "Bad Code", "title": "非法标识", "sort_order": 0, "enabled": true,
	})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("非法分类标识应 400, got %d", rec.Code)
	}
	rec = doWithToken(t, r, adminToken, http.MethodPost, "/api/admin/faq/entries", map[string]any{
		"category_id": liveID, "question": "", "answer": "有答案没问题", "published": true,
	})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("空问题应 400, got %d", rec.Code)
	}
	rec = doWithToken(t, r, adminToken, http.MethodPost, "/api/admin/faq/entries", map[string]any{
		"category_id": 999999, "question": "挂到不存在的分类", "answer": "x", "published": true,
	})
	if rec.Code != http.StatusNotFound {
		t.Fatalf("挂不存在的分类应 404, got %d", rec.Code)
	}

	// ===== 7. 新建 → 更新 → 删除（条目） =====
	rec = doWithToken(t, r, adminToken, http.MethodPost, "/api/admin/faq/entries", map[string]any{
		"category_id": liveID, "question": "新问题", "answer": "新答案", "sort_order": 8, "published": false,
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("新建条目应 200, got %d %s", rec.Code, rec.Body.String())
	}
	var created struct {
		Data struct {
			ID           int    `json:"id"`
			CategoryCode string `json:"category_code"`
			Published    bool   `json:"published"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatalf("解析新建响应失败: %v", err)
	}
	if created.Data.CategoryCode != "account" || created.Data.Published {
		t.Fatalf("新建条目回填不对: %+v", created.Data)
	}
	rec = doWithToken(t, r, adminToken, http.MethodPut, "/api/admin/faq/entries/"+itoa(created.Data.ID), map[string]any{
		"category_id": liveID, "question": "新问题（改）", "answer": "新答案（改）", "sort_order": 8, "published": true,
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("改条目应 200, got %d %s", rec.Code, rec.Body.String())
	}
	// 改完发布后，学员面应能看到了（5 条：原 2 发布 + 这条）
	rec = doWithToken(t, r, stuToken, http.MethodGet, "/api/faq", nil)
	if err := json.Unmarshal(rec.Body.Bytes(), &studentBody); err != nil {
		t.Fatalf("解析响应失败: %v", err)
	}
	if n := len(studentBody.Data.Categories[0].Entries); n != 3 {
		t.Fatalf("发布后学员面应有 3 条, got %d", n)
	}

	// ===== 8. 删分类连带删条目（外键 CASCADE） =====
	rec = doWithToken(t, r, adminToken, http.MethodDelete, "/api/admin/faq/categories/"+itoa(hiddenID), nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("删分类应 200, got %d %s", rec.Code, rec.Body.String())
	}
	var left int64
	if err := db.Model(&model.Faq{}).Where("category_id = ?", hiddenID).Count(&left).Error; err != nil {
		t.Fatalf("计数失败: %v", err)
	}
	if left != 0 {
		t.Fatalf("删分类应连带删除其下条目, 仍剩 %d 条", left)
	}
	// 再删一次：不存在 → 404（哨兵映射，不是 500）
	rec = doWithToken(t, r, adminToken, http.MethodDelete, "/api/admin/faq/categories/"+itoa(hiddenID), nil)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("删不存在的分类应 404, got %d", rec.Code)
	}
}
