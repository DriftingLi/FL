// ADR-0058 契约测试：课程与章节的**按 id 读路径**纳入学员可见性谓词（已发布 + 挂载不变式）。
//
// 覆盖三个读面：课程详情 / 章节详情 / 章节幻灯片。不可见一律按「不存在」返回（404），
// 与搜索的课程/章节分区、收藏的写时校验同一谓词（`CourseVisibleByID` 单点）。
//
// 为什么值得单独立锁：收紧前这三个面只校验「课程/章节存在」，未发布或未挂载的课程
// 只要知道 id 就能被学员读到（题干、正文、PPT 全量）；而发现面一直是过滤的
// ⇒ 「列表/搜索看不到，直链能看」的静默不一致没有任何东西会变红。
package api

import (
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

func TestCourseReadVisibilityContract(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := testutil.NewMemoryDB(t)

	ptr := func(v int) *int { return &v }
	spec := model.Specialty{Code: "read-vis", Name: "读面可见性", SortOrder: 1, Status: 1}
	if err := db.Create(&spec).Error; err != nil {
		t.Fatalf("创建方向失败: %v", err)
	}
	lv := model.CourseLevel{Code: "read-vis-lv", Name: "入门", SortOrder: 1, Status: 1}
	if err := db.Create(&lv).Error; err != nil {
		t.Fatalf("创建等级失败: %v", err)
	}

	// 可见课程：已发布 + 已挂载。
	visible := model.Course{Name: "可见课程", Status: 1,
		SpecialtyID: ptr(spec.SpecialtyID), LevelID: ptr(lv.LevelID), CreatedAt: testutil.Now()}
	if err := db.Create(&visible).Error; err != nil {
		t.Fatalf("创建可见课程失败: %v", err)
	}
	// 未发布课程：已挂载但 status = 0（Course.Status 带 gorm default:1，需显式回写）。
	unpublished := model.Course{Name: "未发布课程", Status: 0,
		SpecialtyID: ptr(spec.SpecialtyID), LevelID: ptr(lv.LevelID), CreatedAt: testutil.Now()}
	if err := db.Create(&unpublished).Error; err != nil {
		t.Fatalf("创建未发布课程失败: %v", err)
	}
	if err := db.Model(&model.Course{}).Where("course_id = ?", unpublished.CourseID).Update("status", 0).Error; err != nil {
		t.Fatalf("置未发布状态失败: %v", err)
	}
	// 未挂载课程：已发布但缺方向 / 等级。
	unmounted := model.Course{Name: "未挂载课程", Status: 1, CreatedAt: testutil.Now()}
	if err := db.Create(&unmounted).Error; err != nil {
		t.Fatalf("创建未挂载课程失败: %v", err)
	}

	visibleCh := model.Chapter{CourseID: visible.CourseID, Title: "可见章节", OrderNum: 1, CreatedAt: testutil.Now()}
	unpublishedCh := model.Chapter{CourseID: unpublished.CourseID, Title: "未发布课程的章节", OrderNum: 1, CreatedAt: testutil.Now()}
	unmountedCh := model.Chapter{CourseID: unmounted.CourseID, Title: "未挂载课程的章节", OrderNum: 1, CreatedAt: testutil.Now()}
	// 孤儿章节：章节行在、所属课程行不在（课程被删而章节未级联删的历史形状）。
	orphanCh := model.Chapter{CourseID: 987654, Title: "孤儿章节", OrderNum: 1, CreatedAt: testutil.Now()}
	for _, ch := range []*model.Chapter{&visibleCh, &unpublishedCh, &unmountedCh, &orphanCh} {
		if err := db.Create(ch).Error; err != nil {
			t.Fatalf("创建章节失败: %v", err)
		}
	}

	user := model.HrwaiUser{Account: "acct_read_vis", Phone: "13800000066", Username: "读面", Status: 1, CreatedAt: testutil.Now()}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("创建用户失败: %v", err)
	}

	cfg := &config.Config{JWTSecretKey: "contract-test-secret", AuthCookie: config.AuthCookieConfig{Name: "hrwai_token"}}
	r := gin.New()
	apiGroup := r.Group("/api")
	deps := newContractDeps(t, db, cfg)
	RegisterCoursesRoutes(apiGroup, deps.RouterDeps(), deps.CourseSvc)

	token, err := security.NewSession(cfg.JWTSecretKey, time.Hour, security.CookieConfig{}).
		Issue(int(user.ID), user.Account, "hrwai_user")
	if err != nil {
		t.Fatalf("签发 token 失败: %v", err)
	}
	do := func(path string) *httptest.ResponseRecorder {
		req, _ := http.NewRequest("GET", path, nil)
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		return w
	}

	for _, tc := range []struct {
		name    string
		path    string
		want    int
		wantMsg string // 非空 = 同时锁 404 面那句对外文案（呈现层统一的证据）
	}{
		// 可见：三个读面都必须照常 200（防「收紧成一律不可见」的假绿）
		{"可见课程详情", fmt.Sprintf("/api/course/%d", visible.CourseID), http.StatusOK, ""},
		{"可见章节详情", fmt.Sprintf("/api/course/%d/chapter/%d", visible.CourseID, visibleCh.ChapterID), http.StatusOK, ""},
		{"可见章节幻灯片", fmt.Sprintf("/api/chapter/%d/slides", visibleCh.ChapterID), http.StatusOK, ""},
		// 未发布：三个读面一律 404
		{"未发布课程详情", fmt.Sprintf("/api/course/%d", unpublished.CourseID), http.StatusNotFound, "课程不存在"},
		{"未发布课程的章节详情", fmt.Sprintf("/api/course/%d/chapter/%d", unpublished.CourseID, unpublishedCh.ChapterID), http.StatusNotFound, "章节不存在"},
		{"未发布课程的章节幻灯片", fmt.Sprintf("/api/chapter/%d/slides", unpublishedCh.ChapterID), http.StatusNotFound, "章节不存在"},
		// 未挂载：三个读面一律 404
		{"未挂载课程详情", fmt.Sprintf("/api/course/%d", unmounted.CourseID), http.StatusNotFound, "课程不存在"},
		{"未挂载课程的章节详情", fmt.Sprintf("/api/course/%d/chapter/%d", unmounted.CourseID, unmountedCh.ChapterID), http.StatusNotFound, "章节不存在"},
		{"未挂载课程的章节幻灯片", fmt.Sprintf("/api/chapter/%d/slides", unmountedCh.ChapterID), http.StatusNotFound, "章节不存在"},
		// 孤儿章节（章节在、所属课程行已不在）：底下那件事实是「课程不存在」，
		// 但本章句端点对外的句子只由**这个端点问的是什么**决定 ⇒ 仍是「章节不存在」。
		// 这一行锁的就是「统一发生在呈现层、由端点显式给出」（ADR-0064 决策 1）。
		{"孤儿章节的幻灯片", fmt.Sprintf("/api/chapter/%d/slides", orphanCh.ChapterID), http.StatusNotFound, "章节不存在"},
	} {
		rec := do(tc.path)
		if rec.Code != tc.want {
			t.Fatalf("%s 期望 %d, got %d: %s", tc.name, tc.want, rec.Code, rec.Body.String())
		}
		if tc.wantMsg == "" {
			continue
		}
		var env struct {
			Message string `json:"message"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
			t.Fatalf("%s 响应不是合法信封: %v (%s)", tc.name, err, rec.Body.String())
		}
		if env.Message != tc.wantMsg {
			// Errorf 而非 Fatalf：一张表里多条文案各自漂移时要一次看全，不是只看第一条。
			t.Errorf("%s 的 404 文案应是「%s」（对象名须与端点问的东西一致），实际「%s」", tc.name, tc.wantMsg, env.Message)
		}
	}
}
