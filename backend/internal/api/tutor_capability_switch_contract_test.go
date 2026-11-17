// Package api 讲师授权链上线契约（#2600 / 规格 E2、F1）：翻转开关之后，讲师的实际可达面
// 由所挂角色的能力集回答，静态能力表对讲师**不再回答**任何东西。
//
// 本文件钉的是「翻转真的生效了吗」——#2599 已把数据层与解析源铺好，但那时 dynamicRoles 里
// 没有讲师，守卫仍走静态表，所以那套测试全绿也证明不了翻转。这里逐条打 HTTP：
//   - 未挂角色的讲师在讲师面端点一律 403（fail closed），但 `/tutor/me/capabilities` 可达
//     且回空集 —— 前端要靠这一条才画得出「还没有讲师权限」页（F1 的核心）；
//   - 四预置角色的能力集决定它打哪些端点成功、打哪些 403；
//   - 静态表里那些键（`tutor.access` 等）对讲师已置空：能力只来自数据层。
package api

import (
	"net/http"
	"strconv"
	"testing"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"forklift-training/internal/authz"
	"forklift-training/internal/config"
	"forklift-training/internal/model"
	"forklift-training/internal/testutil"
)

// tutorSwitchFixture 翻转后的讲师面夹具：四种角色各一位讲师 + 一门带章节的课程。
type tutorSwitchFixture struct {
	t   *testing.T
	r   *gin.Engine
	db  *gorm.DB
	cfg *config.Config

	chapterID int
	courseID  int

	// 四位讲师：全权 / 题库 / 只读 / 未挂角色
	fullTok     string
	bankTok     string
	readonlyTok string
	noneTok     string

	fullID int
	bankID int
}

func newTutorSwitchEnv(t *testing.T) *tutorSwitchFixture {
	t.Helper()
	r, cfg, db := newSlice6Env(t)

	ptr := func(v int) *int { return &v }
	spec := model.Specialty{Code: "switch-cap", Name: "能力切换", SortOrder: 1, Status: 1}
	if err := db.Create(&spec).Error; err != nil {
		t.Fatalf("建方向失败: %v", err)
	}
	lv := model.CourseLevel{Code: "switch-cap-lv", Name: "入门", SortOrder: 1, Status: 1}
	if err := db.Create(&lv).Error; err != nil {
		t.Fatalf("建等级失败: %v", err)
	}
	c := model.Course{Name: "能力切换课程", Status: 1,
		SpecialtyID: ptr(spec.SpecialtyID), LevelID: ptr(lv.LevelID), CreatedAt: testutil.Now()}
	if err := db.Create(&c).Error; err != nil {
		t.Fatalf("建课程失败: %v", err)
	}
	ch := model.Chapter{CourseID: c.CourseID, Title: "能力切换章", Content: "正文",
		ContentType: "text", OrderNum: 1, CreatedAt: testutil.Now()}
	if err := db.Create(&ch).Error; err != nil {
		t.Fatalf("建章节失败: %v", err)
	}

	f := &tutorSwitchFixture{t: t, r: r, db: db, cfg: cfg, chapterID: ch.ChapterID, courseID: c.CourseID}
	f.fullTok, f.fullID = f.seedTutorWithTemplate("全权讲师")
	f.bankTok, f.bankID = f.seedTutorWithTemplate("题库讲师")
	f.readonlyTok, _ = f.seedTutorWithTemplate("只读讲师")
	// 未挂角色：能力集为空（E2 fail closed）
	unassigned := testutil.SeedUnassignedTutor(t, db, "switch_none", "x")
	f.noneTok = slice6Token(t, cfg, unassigned.TutorID, unassigned.Username, "tutor")
	return f
}

// seedTutorWithTemplate 建一位讲师并挂上指定名字的预置角色，返回其令牌与 id。
func (f *tutorSwitchFixture) seedTutorWithTemplate(roleName string) (string, int) {
	f.t.Helper()
	var role model.TutorRole
	if err := f.db.Where("name = ?", roleName).First(&role).Error; err != nil {
		f.t.Fatalf("预置角色 %q 不存在（测试库补建角色链没生效？）: %v", roleName, err)
	}
	tutor := testutil.SeedUnassignedTutor(f.t, f.db, "switch_"+strconv.Itoa(role.RoleID)+"_"+roleName, "x")
	if err := f.db.Model(&model.Tutor{}).Where("tutor_id = ?", tutor.TutorID).
		Update("role_id", role.RoleID).Error; err != nil {
		f.t.Fatalf("挂角色失败: %v", err)
	}
	// 认领本课，否则写路径会先撞认领谓词（本文件验的是能力，不是认领）
	seedTutorCourseClaim(f.t, f.db, tutor.TutorID, f.courseID)
	return slice6Token(f.t, f.cfg, tutor.TutorID, tutor.Username, "tutor"), tutor.TutorID
}

// status 打一条请求返回状态码。
func (f *tutorSwitchFixture) status(method, path, token string, body any) int {
	f.t.Helper()
	rec := doWithToken(f.t, f.r, token, method, path, body)
	return rec.Code
}

// TestUnassignedTutorIsDeniedEverywhereButCanReadOwnCapabilities 未挂角色的讲师：
// 讲师面端点一律 403（fail closed），但「我的能力集」端点必须可达且回空集。
//
// 后者是 F1 的核心：若这个端点也挂能力键，未授权讲师会先撞 403，前端就画不出
// 「还没有讲师权限」页——它只能显示成「加载失败」，把「没授权」冒充成「服务坏了」。
func TestUnassignedTutorIsDeniedEverywhereButCanReadOwnCapabilities(t *testing.T) {
	t.Parallel()
	f := newTutorSwitchEnv(t)

	for _, tc := range []struct {
		name   string
		method string
		path   string
		body   any
	}{
		{"读课程列表", http.MethodGet, "/api/tutor/courses", nil},
		{"读章节", http.MethodGet, "/api/tutor/chapter/" + strconv.Itoa(f.chapterID), nil},
		{"改章节", http.MethodPut, "/api/tutor/chapter/" + strconv.Itoa(f.chapterID), map[string]any{"title": "x"}},
		{"文件库列表", http.MethodGet, "/api/tutor/files", nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if code := f.status(tc.method, tc.path, f.noneTok, tc.body); code != http.StatusForbidden {
				t.Fatalf("%s 未挂角色应 403, got %d", tc.name, code)
			}
		})
	}

	t.Run("我的能力集可达且回空集", func(t *testing.T) {
		rec := doWithToken(t, f.r, f.noneTok, http.MethodGet, "/api/tutor/me/capabilities", nil)
		if rec.Code != http.StatusOK {
			t.Fatalf("未挂角色取能力集应 200（只挂登录守卫）, got %d: %s", rec.Code, rec.Body.String())
		}
		data := decodeEnvelopeData(t, rec.Body.String())
		if granted, _ := data["granted"].(bool); granted {
			t.Fatalf("未挂角色 granted 应为 false, got %v", data["granted"])
		}
		caps, _ := data["capabilities"].([]any)
		if len(caps) != 0 {
			t.Fatalf("未挂角色能力集应为空, got %v", caps)
		}
	})
}

// TestFourTemplatesDecideTutorReachability 四预置角色的能力集决定它打哪些端点成功、打哪些 403。
//
// 这条是「四模板实际可见面」在**后端**的那一半（前端页面集由 config/__tests__/tutorLanding.spec.ts 锁）。
func TestFourTemplatesDecideTutorReachability(t *testing.T) {
	t.Parallel()
	f := newTutorSwitchEnv(t)

	chapterPath := "/api/tutor/chapter/" + strconv.Itoa(f.chapterID)
	readChapter := func(tok string) int { return f.status(http.MethodGet, chapterPath, tok, nil) }
	writeChapter := func(tok string) int {
		return f.status(http.MethodPut, chapterPath, tok, map[string]any{"title": "改标题"})
	}
	listFiles := func(tok string) int { return f.status(http.MethodGet, "/api/tutor/files", tok, nil) }

	// 全权讲师：7 键全有 ⇒ 读写与文件库全通
	if code := readChapter(f.fullTok); code != http.StatusOK {
		t.Fatalf("全权讲师读章节应 200, got %d", code)
	}
	if code := writeChapter(f.fullTok); code != http.StatusOK {
		t.Fatalf("全权讲师写章节应 200, got %d", code)
	}
	if code := listFiles(f.fullTok); code != http.StatusOK {
		t.Fatalf("全权讲师读文件库应 200, got %d", code)
	}

	// 题库讲师：有 tutor.access（读通），没有 course.author / tutorfile.manage（写与库 403）
	if code := readChapter(f.bankTok); code != http.StatusOK {
		t.Fatalf("题库讲师读章节应 200, got %d", code)
	}
	if code := writeChapter(f.bankTok); code != http.StatusForbidden {
		t.Fatalf("题库讲师写章节应 403（无 course.author）, got %d", code)
	}
	if code := listFiles(f.bankTok); code != http.StatusForbidden {
		t.Fatalf("题库讲师读文件库应 403（无 tutorfile.manage）, got %d", code)
	}

	// 只读讲师：只有 tutor.access ⇒ 读通、写 403、库 403
	if code := readChapter(f.readonlyTok); code != http.StatusOK {
		t.Fatalf("只读讲师读章节应 200, got %d", code)
	}
	if code := writeChapter(f.readonlyTok); code != http.StatusForbidden {
		t.Fatalf("只读讲师写章节应 403, got %d", code)
	}
	if code := listFiles(f.readonlyTok); code != http.StatusForbidden {
		t.Fatalf("只读讲师读文件库应 403, got %d", code)
	}
}

// TestTutorStaticTableNoLongerAnswers 静态能力表对讲师已置空：能力只来自数据层。
//
// 这条用**没有挂任何角色**的讲师反证：若静态表还在回答讲师，它就该拿到 `tutor.access`
// 从而读通课程列表。它 403 就说明「讲师的可达面确实改由数据层回答」。
// 同时断言 authz 侧的两个事实，避免本文件与生成物漂移。
func TestTutorStaticTableNoLongerAnswers(t *testing.T) {
	t.Parallel()
	f := newTutorSwitchEnv(t)

	if code := f.status(http.MethodGet, "/api/tutor/courses", f.noneTok, nil); code != http.StatusForbidden {
		t.Fatalf("未挂角色讲师读课程列表应 403（静态表已不回答讲师）, got %d", code)
	}

	// authz 侧：讲师是动态角色，且静态表里讲师侧键一枚都不剩
	if !authz.IsDynamicRole(authz.RoleTutor) {
		t.Fatal("讲师应为动态角色（dynamicRoles 已翻转）")
	}
	if caps := authz.Capabilities(authz.RoleTutor); len(caps) != 0 {
		t.Fatalf("讲师静态能力行应为空, got %v", caps)
	}
	for _, c := range authz.ProtectedTutorCapabilities() {
		if authz.Has(authz.RoleTutor, c) {
			t.Fatalf("讲师不应再从静态表拿到 %q", c)
		}
	}
}

// TestTutorCapabilitiesEndpointRoleGuard 该端点只收讲师：管理员与学员都 403。
//
// 与 admin 侧同名端点同形（`CurrentRole != 目标角色 → 403`）：它答的是「**我**的能力集」，
// 别的角色打进来只会拿到一份与它无关的空集，那种「看起来成功」比 403 更容易误导。
func TestTutorCapabilitiesEndpointRoleGuard(t *testing.T) {
	t.Parallel()
	f := newTutorSwitchEnv(t)

	adminTok := adminTokenWithAccount(t, f.cfg, f.db, "switch_admin")
	if code := f.status(http.MethodGet, "/api/tutor/me/capabilities", adminTok, nil); code != http.StatusForbidden {
		t.Fatalf("管理员打讲师能力集端点应 403, got %d", code)
	}
	if code := f.status(http.MethodGet, "/api/tutor/me/capabilities", "", nil); code != http.StatusUnauthorized {
		t.Fatalf("匿名应 401, got %d", code)
	}
}
