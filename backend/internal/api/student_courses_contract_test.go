// ADR-0017 契约测试：学习位置与章节完成状态。
//   - POST /api/course/:id/progress 支持 duration_seconds / video_position / completed（旧 body 兼容）
//   - GET /api/student/courses 我的课程（含 continue_learning 与最后学习位置）
//   - GET /api/student/courses/:course_id 单课程学习详情（每章进度/位置/完成）
//   - GET /api/course/:id 详情增强（is_enrolled / completed_chapters / last_*）
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

func TestLearningPositionContract(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := testutil.NewMemoryDB(t)

	ptr := func(v int) *int { return &v }
	spec := model.Specialty{Code: "maintenance", Name: "维修", SortOrder: 1, Status: 1}
	if err := db.Create(&spec).Error; err != nil {
		t.Fatalf("创建方向失败: %v", err)
	}
	lv := model.CourseLevel{Code: "beginner", Name: "入门", SortOrder: 1, Status: 1}
	if err := db.Create(&lv).Error; err != nil {
		t.Fatalf("创建等级失败: %v", err)
	}
	course := model.Course{Name: "契约课程", CoverImage: "/static/covers/c1.png", Status: 1,
		SpecialtyID: ptr(spec.SpecialtyID), LevelID: ptr(lv.LevelID), CreatedAt: testutil.Now()}
	if err := db.Create(&course).Error; err != nil {
		t.Fatalf("创建课程失败: %v", err)
	}
	ch1 := model.Chapter{CourseID: course.CourseID, Title: "第一章", Duration: 10, OrderNum: 1, CreatedAt: testutil.Now()}
	if err := db.Create(&ch1).Error; err != nil {
		t.Fatalf("创建章节失败: %v", err)
	}
	ch2 := model.Chapter{CourseID: course.CourseID, Title: "第二章", Duration: 5, OrderNum: 2, CreatedAt: testutil.Now()}
	if err := db.Create(&ch2).Error; err != nil {
		t.Fatalf("创建章节失败: %v", err)
	}

	cfg := &config.Config{
		JWTSecretKey: "contract-test-secret",
		AuthCookie:   config.AuthCookieConfig{Name: "hrwai_token"},
	}
	r := gin.New()
	api := r.Group("/api")
	deps := newContractDeps(t, db, cfg)
	RegisterCoursesRoutes(api, deps.RouterDeps(), deps.CourseSvc)
	RegisterStudentRoutes(api, deps.RouterDeps(), deps.StudentSvc)

	const studentID = 7
	token, err := security.NewSession(cfg.JWTSecretKey, time.Hour, security.CookieConfig{}).
		Issue(studentID, "13800000007", "hrwai_user")
	if err != nil {
		t.Fatalf("签发 token 失败: %v", err)
	}

	do := func(method, path string, body any) *httptest.ResponseRecorder {
		var req *http.Request
		if body != nil {
			b, _ := json.Marshal(body)
			req, _ = http.NewRequest(method, path, bytes.NewReader(b))
			req.Header.Set("Content-Type", "application/json")
		} else {
			req, _ = http.NewRequest(method, path, nil)
		}
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		return w
	}
	progressPath := fmt.Sprintf("/api/course/%d/progress", course.CourseID)

	// 1. 秒级时长 + 播放位置上报：95 秒 → 2 分钟累计，未完成。
	rec := do(http.MethodPost, progressPath, map[string]any{
		"chapter_id": ch1.ChapterID, "duration_seconds": 95, "video_position": 120,
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("进度上报期望 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var prog struct {
		Data struct {
			StudyDuration     int64   `json:"study_duration"`
			CompletedChapters int64   `json:"completed_chapters"`
			Progress          float64 `json:"progress"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &prog); err != nil {
		t.Fatalf("解析进度响应失败: %v", err)
	}
	if prog.Data.StudyDuration != 2 {
		t.Fatalf("95 秒应换算 2 分钟累计, got %d", prog.Data.StudyDuration)
	}
	if prog.Data.CompletedChapters != 0 {
		t.Fatalf("未达阈值不应完成, got %d", prog.Data.CompletedChapters)
	}

	// 2. 旧 body 兼容（仅 chapter_id + duration 分钟）：10 分钟达章节阈值自动完成。
	rec = do(http.MethodPost, progressPath, map[string]any{"chapter_id": ch1.ChapterID, "duration": 10})
	if rec.Code != http.StatusOK {
		t.Fatalf("旧 body 上报期望 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &prog); err != nil {
		t.Fatalf("解析进度响应失败: %v", err)
	}
	if prog.Data.CompletedChapters != 1 {
		t.Fatalf("时长达标应完成 1 章, got %d", prog.Data.CompletedChapters)
	}
	if prog.Data.Progress != 50 {
		t.Fatalf("1/2 章完成课程进度应为 50, got %v", prog.Data.Progress)
	}

	// 3. 显式完成第二章（completed）+ 位置 30。
	rec = do(http.MethodPost, progressPath, map[string]any{
		"chapter_id": ch2.ChapterID, "completed": true, "video_position": 30,
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("显式完成期望 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &prog); err != nil {
		t.Fatalf("解析进度响应失败: %v", err)
	}
	if prog.Data.CompletedChapters != 2 || prog.Data.Progress != 100 {
		t.Fatalf("全部完成应 2 章/100, got %d/%v", prog.Data.CompletedChapters, prog.Data.Progress)
	}

	// 4. 我的课程：位置与 continue_learning。
	rec = do(http.MethodGet, "/api/student/courses", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("我的课程期望 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var mine struct {
		Data struct {
			Courses          []map[string]any `json:"courses"`
			ContinueLearning map[string]any   `json:"continue_learning"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &mine); err != nil {
		t.Fatalf("解析我的课程失败: %v", err)
	}
	if len(mine.Data.Courses) != 1 {
		t.Fatalf("应返回 1 门课程, got %d", len(mine.Data.Courses))
	}
	item := mine.Data.Courses[0]
	if item["course_name"] != "契约课程" || item["cover"] != "/static/covers/c1.png" {
		t.Fatalf("课程元信息回填错误: %+v", item)
	}
	if item["total_chapters"] != float64(2) || item["completed_chapters"] != float64(2) {
		t.Fatalf("章节数/完成数错误: %+v", item)
	}
	if item["last_chapter_id"] != float64(ch2.ChapterID) || item["last_position"] != float64(30) {
		t.Fatalf("最后学习位置错误: %+v", item)
	}
	if item["last_chapter_title"] != "第二章" {
		t.Fatalf("最后章节标题错误: %+v", item)
	}
	if item["last_studied_at"] == nil || item["last_studied_at"] == "" {
		t.Fatalf("最后学习时间不应为空: %+v", item)
	}
	if mine.Data.ContinueLearning["course_id"] != float64(course.CourseID) {
		t.Fatalf("continue_learning 应指向最新学习课程: %+v", mine.Data.ContinueLearning)
	}

	// 5. 单课程学习详情：每章状态与播放位置。
	rec = do(http.MethodGet, fmt.Sprintf("/api/student/courses/%d", course.CourseID), nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("课程学习详情期望 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var detail struct {
		Data struct {
			CompletedChapters float64          `json:"completed_chapters"`
			Chapters          []map[string]any `json:"chapters"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &detail); err != nil {
		t.Fatalf("解析课程学习详情失败: %v", err)
	}
	if len(detail.Data.Chapters) != 2 {
		t.Fatalf("应返回 2 个章节, got %d", len(detail.Data.Chapters))
	}
	if detail.Data.Chapters[0]["completed"] != true || detail.Data.Chapters[0]["video_position"] != float64(120) {
		t.Fatalf("第一章状态错误: %+v", detail.Data.Chapters[0])
	}
	if detail.Data.Chapters[1]["completed"] != true || detail.Data.Chapters[1]["video_position"] != float64(30) {
		t.Fatalf("第二章状态错误: %+v", detail.Data.Chapters[1])
	}

	// 6. 课程详情增强：学习位置字段。
	rec = do(http.MethodGet, fmt.Sprintf("/api/course/%d", course.CourseID), nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("课程详情期望 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var cd struct {
		Data struct {
			IsEnrolled        bool    `json:"is_enrolled"`
			CompletedChapters float64 `json:"completed_chapters"`
			LastChapterID     float64 `json:"last_chapter_id"`
			LastPosition      float64 `json:"last_position"`
			LastStudiedAt     string  `json:"last_studied_at"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &cd); err != nil {
		t.Fatalf("解析课程详情失败: %v", err)
	}
	if !cd.Data.IsEnrolled || cd.Data.CompletedChapters != 2 {
		t.Fatalf("is_enrolled/completed_chapters 错误: %+v", cd.Data)
	}
	if cd.Data.LastChapterID != float64(ch2.ChapterID) || cd.Data.LastPosition != 30 {
		t.Fatalf("last_chapter_id/last_position 错误: %+v", cd.Data)
	}
	if cd.Data.LastStudiedAt == "" {
		t.Fatal("last_studied_at 不应为空")
	}
}

// 未登录/未学学员：我的课程空信封、课程详情零值学习位置。
func TestLearningPositionEmptyContract(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := testutil.NewMemoryDB(t)

	ptr := func(v int) *int { return &v }
	spec := model.Specialty{Code: "m", Name: "维修", SortOrder: 1, Status: 1}
	db.Create(&spec)
	lv := model.CourseLevel{Code: "b", Name: "入门", SortOrder: 1, Status: 1}
	db.Create(&lv)
	course := model.Course{Name: "空契约课程", Status: 1,
		SpecialtyID: ptr(spec.SpecialtyID), LevelID: ptr(lv.LevelID), CreatedAt: testutil.Now()}
	db.Create(&course)

	cfg := &config.Config{
		JWTSecretKey: "contract-test-secret",
		AuthCookie:   config.AuthCookieConfig{Name: "hrwai_token"},
	}
	r := gin.New()
	api := r.Group("/api")
	deps := newContractDeps(t, db, cfg)
	RegisterCoursesRoutes(api, deps.RouterDeps(), deps.CourseSvc)
	RegisterStudentRoutes(api, deps.RouterDeps(), deps.StudentSvc)

	token, err := security.NewSession(cfg.JWTSecretKey, time.Hour, security.CookieConfig{}).
		Issue(9, "13800000009", "hrwai_user")
	if err != nil {
		t.Fatalf("签发 token 失败: %v", err)
	}

	req, _ := http.NewRequest(http.MethodGet, "/api/student/courses", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("期望 200, got %d: %s", w.Code, w.Body.String())
	}
	var mine struct {
		Data struct {
			Courses          []map[string]any `json:"courses"`
			ContinueLearning map[string]any   `json:"continue_learning"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &mine); err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	if len(mine.Data.Courses) != 0 {
		t.Fatalf("未学学员应无课程, got %d", len(mine.Data.Courses))
	}
	if mine.Data.ContinueLearning != nil {
		t.Fatalf("未学学员 continue_learning 应为 null, got %v", mine.Data.ContinueLearning)
	}

	req, _ = http.NewRequest(http.MethodGet, fmt.Sprintf("/api/course/%d", course.CourseID), nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	var cd struct {
		Data struct {
			IsEnrolled        bool    `json:"is_enrolled"`
			CompletedChapters float64 `json:"completed_chapters"`
			LastChapterID     any     `json:"last_chapter_id"`
			LastPosition      float64 `json:"last_position"`
			LastStudiedAt     string  `json:"last_studied_at"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &cd); err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	if cd.Data.IsEnrolled || cd.Data.CompletedChapters != 0 || cd.Data.LastPosition != 0 || cd.Data.LastStudiedAt != "" {
		t.Fatalf("未学学员学习位置应为零值: %+v", cd.Data)
	}
}
