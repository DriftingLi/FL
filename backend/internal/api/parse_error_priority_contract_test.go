// 契约测（ADR-0062 决策 8 / 票8）：`*ParseError` 恒优先于**无条件条目**（`sentinel == nil`）。
//
// 病根（实测，非假想）：`renderError` 的旧判定序先按声明顺序扫 entries，`sentinel == nil` 的条目
// 无条件命中一切错误（含 `*ParseError`）⇒ 挂 `WithSuccess(ok, 500)` / `errStatusAllPrefix(500,…)` /
// `errStatusAll(404)` 的端点把 400 类参数错误答成 500 或 404，连自己的 `@Failure 400` 注解都违反
// （ADR-0062 实施回记「票8 的一个实例」：删掉 `base_url` 打 PUT /admin/ai-configs/:id 即复现）。
//
// 取样口径：三族各一例（骨架层的完整判定序见 errstatus_test.go；逐端点的错误表判定见本文件后半的
// regenerate 三例与 shop_sku 专项）。断言走**真实路由**——「注解写了 400 而实际 500」这类矛盾
// 只有打一次才会露出来。
package api

import (
	"net/http"
	"strconv"
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

// parseErrFixture 全量路由 + 一名学员与一名管理员的 token。
type parseErrFixture struct {
	db         *gorm.DB
	token      string
	adminToken string
}

func parseErrRouter(t *testing.T) (*gin.Engine, *parseErrFixture) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db := testutil.NewMemoryDB(t)
	cfg := &config.Config{
		JWTSecretKey:    "parse-priority-secret",
		JWTExpiresHours: 2,
		AuthCookie:      config.AuthCookieConfig{Name: "hrwai_token"},
	}
	r := NewRouter(newContractDeps(t, db, cfg))
	return r, &parseErrFixture{
		db:         db,
		token:      parseErrStudentToken(t, db, cfg, "parse_priority_stu"),
		adminToken: parseErrAdminToken(t, db, cfg, "parsePriorityAdmin"),
	}
}

// TestParseErrorBeatsUnconditionalEntries 无条件条目三族：参数错误必须回自己的 400 与自己的文案。
// forbid 钉的是「无条件条目那一层的痕迹」（人读前缀）不得再覆盖解析错误文案。
func TestParseErrorBeatsUnconditionalEntries(t *testing.T) {
	r, fx := parseErrRouter(t)

	cases := []struct {
		name    string
		method  string
		path    string
		body    any
		admin   bool
		wantMsg string
		forbid  string
	}{
		{
			name: "WithSuccess(...,500) 族：query 参数错误", method: http.MethodGet,
			path: "/api/admin/courses?filter=nope", admin: true, wantMsg: "filter 仅支持 hot|featured|all",
		},
		{
			name: "errStatusAllPrefix(500) 族：body binding:required 缺失", method: http.MethodPost,
			path: "/api/admin/ai-configs", body: map[string]any{"name": "只给名字"}, admin: true,
			wantMsg: "请求参数错误", forbid: "创建失败",
		},
		{
			name: "哨兵 + 尾部无条件条目（ai-config 更新）：ADR-0062 实施回记的实测复现", method: http.MethodPut,
			path: "/api/admin/ai-configs/999999", body: map[string]any{"name": "改个不存在的配置"}, admin: true,
			wantMsg: "请求参数错误", forbid: "更新失败",
		},
		{
			name: "errStatusAll(404) 族：路径参数非数字", method: http.MethodPut,
			path: "/api/admin/hrwai-users/abc/status", admin: true, wantMsg: "用户ID无效",
		},
		{
			name: "WithSuccess(...,404) 族：路径参数非数字", method: http.MethodGet,
			path: "/api/chapter/abc/slides", wantMsg: "章节ID无效",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			tok := fx.token
			if c.admin {
				tok = fx.adminToken
			}
			rec := doWithToken(t, r, tok, c.method, c.path, c.body)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("参数错误必须 400（*ParseError 优先于无条件条目）: got %d %s", rec.Code, rec.Body.String())
			}
			if !strings.Contains(rec.Body.String(), c.wantMsg) {
				t.Fatalf("文案必须是解析错误自己的话（%q）: %s", c.wantMsg, rec.Body.String())
			}
			if c.forbid != "" && strings.Contains(rec.Body.String(), c.forbid) {
				t.Fatalf("无条件条目的人读前缀（%q）不得覆盖解析错误: %s", c.forbid, rec.Body.String())
			}
		})
	}
}

// ===== 另一种病：errStatusAll(404) 把「不是 ParseError 的一切」都答 404 =====

// regenerateFixture 建「可见课程 + 章节」，paid=true 时课程按 100 分收费（⇒ 读面需要权益）。
type regenerateFixture struct {
	chapterID int
	courseID  int
	regenURL  string
}

func newRegenerateFixture(t *testing.T, db *gorm.DB, paid bool) regenerateFixture {
	t.Helper()
	spec := model.Specialty{Code: "reg-gen", Name: "重新生成", SortOrder: 1, Status: 1}
	if err := db.Create(&spec).Error; err != nil {
		t.Fatalf("建专业方向失败: %v", err)
	}
	lv := model.CourseLevel{Code: "reg-gen-lv", Name: "入门", SortOrder: 1, Status: 1}
	if err := db.Create(&lv).Error; err != nil {
		t.Fatalf("建课程等级失败: %v", err)
	}
	course := model.Course{Name: "叉车发动机拆装", Status: 1,
		SpecialtyID: &spec.SpecialtyID, LevelID: &lv.LevelID, CreatedAt: testutil.Now()}
	if paid {
		price := 100
		course.PointsPrice = &price
	}
	if err := db.Create(&course).Error; err != nil {
		t.Fatalf("建课程失败: %v", err)
	}
	ch := model.Chapter{CourseID: course.CourseID, Title: "曲轴箱清洗", OrderNum: 1, CreatedAt: testutil.Now()}
	if err := db.Create(&ch).Error; err != nil {
		t.Fatalf("建章节失败: %v", err)
	}
	return regenerateFixture{
		chapterID: ch.ChapterID, courseID: course.CourseID,
		regenURL: "/api/chapter/" + strconv.Itoa(ch.ChapterID) + "/slides/regenerate",
	}
}

// TestRegenerateSlidesGateStillRenders404 门禁（未兑换的付费课程）仍按「不存在」返回 404
// （ADR-0062 决策 3：越权按不存在，不泄漏存在性）。这条是改判后**必须不变**的那一半。
func TestRegenerateSlidesGateStillRenders404(t *testing.T) {
	r, fx := parseErrRouter(t)
	fixture := newRegenerateFixture(t, fx.db, true)
	rec := doWithToken(t, r, fx.token, http.MethodPost, fixture.regenURL, nil)
	if rec.Code != http.StatusNotFound || !strings.Contains(rec.Body.String(), `"code":404`) {
		t.Fatalf("未兑换的付费课程内容必须 404: got %d %s", rec.Code, rec.Body.String())
	}
}

// TestRegenerateSlidesDownstreamFailureRenders500 下游失败（该章节没有 PPT / 转图失败）不是「不存在」：
// 旧形状 `WithSuccess(ok, 404)` 把门禁与下游故障压成同一个 404 ⇒ 门禁本身无测可建
// （ADR-0062 复核登记「regenerate 的门禁建不了锁」）。
func TestRegenerateSlidesDownstreamFailureRenders500(t *testing.T) {
	r, fx := parseErrRouter(t)
	fixture := newRegenerateFixture(t, fx.db, true)
	if rec := doWithToken(t, r, fx.token, http.MethodPost,
		"/api/points/shop/course/"+strconv.Itoa(fixture.courseID)+"/redeem", nil); rec.Code != http.StatusOK {
		t.Fatalf("兑换付费课程应 200: got %d %s", rec.Code, rec.Body.String())
	}
	rec := doWithToken(t, r, fx.token, http.MethodPost, fixture.regenURL, nil)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("无 PPT / 转图失败必须 500（不得伪装成「不存在」）: got %d %s", rec.Code, rec.Body.String())
	}
}

// TestRegenerateSlidesDBFailureRenders500 权益查询本身查不动（删 user_entitlement 表 = 真实驱动错误）
// 不得被读成「这个章节不存在」⇒ 必须 500（ADR-0062 票6「查不动 ≠ 查得空」在错误面的另一半）。
func TestRegenerateSlidesDBFailureRenders500(t *testing.T) {
	r, fx := parseErrRouter(t)
	fixture := newRegenerateFixture(t, fx.db, true)
	if err := fx.db.Migrator().DropTable(&model.UserEntitlement{}); err != nil {
		t.Fatalf("注入故障（删权益表）失败: %v", err)
	}
	rec := doWithToken(t, r, fx.token, http.MethodPost, fixture.regenURL, nil)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("权益查不动必须 500，不得答 404: got %d %s", rec.Code, rec.Body.String())
	}
}

// ===== token 助手（本文件自带，不改既有夹具） =====

func parseErrStudentToken(t *testing.T, db *gorm.DB, cfg *config.Config, account string) string {
	t.Helper()
	pwd, err := service.HashPassword("student123")
	if err != nil {
		t.Fatalf("hash password failed: %v", err)
	}
	stu := testutil.SeedStudent(t, db, account, pwd)
	if err := db.Model(&model.HrwaiUser{}).Where("id = ?", stu.ID).
		Update("points_balance", 5000).Error; err != nil {
		t.Fatalf("预置余额失败: %v", err)
	}
	token, err := security.NewSession(cfg.JWTSecretKey, time.Hour, security.CookieConfig{Name: cfg.AuthCookie.Name}).
		Issue(stu.ID, stu.Username, "hrwai_user")
	if err != nil {
		t.Fatalf("签发 token 失败: %v", err)
	}
	return token
}

func parseErrAdminToken(t *testing.T, db *gorm.DB, cfg *config.Config, username string) string {
	t.Helper()
	admin := testutil.SeedAdmin(t, db, username, "x")
	token, err := security.NewSession(cfg.JWTSecretKey, time.Hour, security.CookieConfig{Name: cfg.AuthCookie.Name}).
		Issue(admin.AdminID, admin.Username, "admin")
	if err != nil {
		t.Fatalf("签发 token 失败: %v", err)
	}
	return token
}
