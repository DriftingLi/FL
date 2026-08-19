// ADR-0018 契约测试：通用收藏 + 全局搜索 + 学习资料聚合。
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

// seedP1Env 播种收藏/搜索/资料共用数据：
// 已发布挂载课程（含章节+附件）、未发布课程（隐藏）、已发布/草稿题目、
// 已发布/下线精选、论坛帖（作者 + 手机号保证唯一）。
func seedP1Env(t *testing.T) (*gin.Engine, *config.Config, int) {
	t.Helper()
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
	course := model.Course{Name: "液压传动维修", CoverImage: "/static/covers/h.png", Description: "液压系统原理与维修",
		Status: 1, SpecialtyID: ptr(spec.SpecialtyID), LevelID: ptr(lv.LevelID), CreatedAt: testutil.Now()}
	if err := db.Create(&course).Error; err != nil {
		t.Fatalf("创建课程失败: %v", err)
	}
	// 注意：Course.Status 带 gorm default:1，零值 0 在 Create 时会被默认值覆盖，需显式置 0。
	unpublished := model.Course{Name: "液压内部课程", Status: 0,
		SpecialtyID: ptr(spec.SpecialtyID), LevelID: ptr(lv.LevelID), CreatedAt: testutil.Now()}
	if err := db.Create(&unpublished).Error; err != nil {
		t.Fatalf("创建未发布课程失败: %v", err)
	}
	if err := db.Model(&model.Course{}).Where("course_id = ?", unpublished.CourseID).Update("status", 0).Error; err != nil {
		t.Fatalf("置未发布状态失败: %v", err)
	}
	ch := model.Chapter{CourseID: course.CourseID, Title: "液压泵拆装", OrderNum: 1, CreatedAt: testutil.Now()}
	if err := db.Create(&ch).Error; err != nil {
		t.Fatalf("创建章节失败: %v", err)
	}
	file := model.ChapterFile{ChapterID: &ch.ChapterID, FileName: "液压手册.pdf",
		FileURL: "/static/uploads/chapters/h.pdf", ContentType: "document", FileSize: 1024, CreatedAt: testutil.Now()}
	if err := db.Create(&file).Error; err != nil {
		t.Fatalf("创建附件失败: %v", err)
	}
	legacy := model.ChapterFile{FileName: "legacy.pdf", FileURL: "/static/legacy.pdf", CreatedAt: testutil.Now()}
	if err := db.Create(&legacy).Error; err != nil {
		t.Fatalf("创建 legacy 附件失败: %v", err)
	}
	published := model.Question{Type: "single", Content: "液压油温过高的原因不包括", Status: "published", CreatedAt: testutil.Now()}
	if err := db.Create(&published).Error; err != nil {
		t.Fatalf("创建题目失败: %v", err)
	}
	draft := model.Question{Type: "single", Content: "液压草稿题（不可见）", Status: "draft", CreatedAt: testutil.Now()}
	if err := db.Create(&draft).Error; err != nil {
		t.Fatalf("创建草稿题失败: %v", err)
	}
	featured := model.FeaturedContent{Title: "液压维保资讯", Summary: "行业液压动态", Status: 1, CreatedAt: testutil.Now()}
	if err := db.Create(&featured).Error; err != nil {
		t.Fatalf("创建精选失败: %v", err)
	}
	offline := model.FeaturedContent{Title: "下线液压内容", Status: 0, CreatedAt: testutil.Now()}
	if err := db.Create(&offline).Error; err != nil {
		t.Fatalf("创建下线精选失败: %v", err)
	}
	user := model.HrwaiUser{Account: "acct_p1", Phone: "13800000099", Username: "收藏家", Status: 1, CreatedAt: testutil.Now()}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("创建用户失败: %v", err)
	}
	now := testutil.Now()
	topic := model.ForumTopic{UserID: int(user.ID), Title: "液压求助帖", Content: "求液压维修经验", CreatedAt: now, UpdatedAt: now}
	if err := db.Create(&topic).Error; err != nil {
		t.Fatalf("创建帖子失败: %v", err)
	}

	cfg := &config.Config{
		JWTSecretKey: "contract-test-secret",
		AuthCookie:   config.AuthCookieConfig{Name: "hrwai_token"},
	}
	r := gin.New()
	api := r.Group("/api")
	deps := newContractDeps(t, db, cfg)
	RegisterFavoriteRoutes(api, deps.RouterDeps(), deps.FavoriteSvc)
	RegisterSearchRoutes(api, deps.RouterDeps(), deps.SearchSvc)
	RegisterMaterialRoutes(api, deps.RouterDeps(), deps.MaterialSvc)

	return r, cfg, int(user.ID)
}

func p1Request(r *gin.Engine, token, method, path string, body any) *httptest.ResponseRecorder {
	var req *http.Request
	if body != nil {
		b, _ := json.Marshal(body)
		req, _ = http.NewRequest(method, path, bytes.NewReader(b))
		req.Header.Set("Content-Type", "application/json")
	} else {
		req, _ = http.NewRequest(method, path, nil)
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestFavoritesContract(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r, cfg, userID := seedP1Env(t)

	token, err := security.NewSession(cfg.JWTSecretKey, time.Hour, security.CookieConfig{}).
		Issue(userID, "acct_p1", "hrwai_user")
	if err != nil {
		t.Fatalf("签发 token 失败: %v", err)
	}

	// 收藏课程（幂等）+ check。
	rec := p1Request(r, token, http.MethodPost, "/api/favorites", map[string]any{"target_type": "course", "target_id": 1})
	if rec.Code != http.StatusCreated {
		t.Fatalf("收藏课程期望 201, got %d: %s", rec.Code, rec.Body.String())
	}
	var added struct {
		Data struct {
			FavoriteID int64  `json:"favorite_id"`
			Title      string `json:"title"`
			Cover      string `json:"cover"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &added); err != nil {
		t.Fatalf("解析收藏响应失败: %v", err)
	}
	if added.Data.Title != "液压传动维修" || added.Data.Cover != "/static/covers/h.png" {
		t.Fatalf("收藏快照错误: %+v", added.Data)
	}
	rec = p1Request(r, token, http.MethodPost, "/api/favorites", map[string]any{"target_type": "course", "target_id": 1})
	if rec.Code != http.StatusCreated {
		t.Fatalf("重复收藏应幂等 201, got %d", rec.Code)
	}

	// 帖子/章节/题目/精选收藏 + 类型过滤列表。
	for _, body := range []map[string]any{
		{"target_type": "topic", "target_id": 1},
		{"target_type": "chapter", "target_id": 1},
		{"target_type": "question", "target_id": 1},
		{"target_type": "featured", "target_id": 1},
	} {
		rec := p1Request(r, token, http.MethodPost, "/api/favorites", body)
		if rec.Code != http.StatusCreated {
			t.Fatalf("收藏 %+v 期望 201, got %d: %s", body, rec.Code, rec.Body.String())
		}
	}
	rec = p1Request(r, token, http.MethodGet, "/api/favorites", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("收藏列表期望 200, got %d", rec.Code)
	}
	var list struct {
		Data struct {
			Total     int64 `json:"total"`
			Favorites []struct {
				TargetType string `json:"target_type"`
				Title      string `json:"title"`
			} `json:"favorites"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil {
		t.Fatalf("解析收藏列表失败: %v", err)
	}
	if list.Data.Total != 5 || len(list.Data.Favorites) != 5 {
		t.Fatalf("收藏总数应为 5, got %d/%d", list.Data.Total, len(list.Data.Favorites))
	}
	rec = p1Request(r, token, http.MethodGet, "/api/favorites?target_type=topic", nil)
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil {
		t.Fatalf("解析过滤列表失败: %v", err)
	}
	if list.Data.Total != 1 || list.Data.Favorites[0].Title != "液压求助帖" {
		t.Fatalf("帖子收藏过滤错误: %+v", list.Data)
	}

	// check 状态。
	rec = p1Request(r, token, http.MethodGet, "/api/favorites/check?target_type=course&target_id=1", nil)
	var check struct {
		Data struct {
			Favorited bool `json:"favorited"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &check); err != nil {
		t.Fatalf("解析 check 失败: %v", err)
	}
	if !check.Data.Favorited {
		t.Fatal("已收藏应 favorited=true")
	}

	// 不可见目标拒绝：未发布课程 / 草稿题 / 下线精选 / 未知类型。
	for _, body := range []map[string]any{
		{"target_type": "course", "target_id": 2},
		{"target_type": "question", "target_id": 2},
		{"target_type": "featured", "target_id": 2},
		{"target_type": "unknown", "target_id": 1},
	} {
		rec := p1Request(r, token, http.MethodPost, "/api/favorites", body)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("不可收藏目标 %+v 期望 400, got %d", body, rec.Code)
		}
	}

	// 取消收藏 → check false；重复删除 400。
	rec = p1Request(r, token, http.MethodDelete, fmt.Sprintf("/api/favorites/%d", added.Data.FavoriteID), nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("取消收藏期望 200, got %d", rec.Code)
	}
	rec = p1Request(r, token, http.MethodGet, "/api/favorites/check?target_type=course&target_id=1", nil)
	if err := json.Unmarshal(rec.Body.Bytes(), &check); err != nil {
		t.Fatalf("解析 check 失败: %v", err)
	}
	if check.Data.Favorited {
		t.Fatal("取消后应 favorited=false")
	}
	rec = p1Request(r, token, http.MethodDelete, fmt.Sprintf("/api/favorites/%d", added.Data.FavoriteID), nil)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("重复删除期望 400, got %d", rec.Code)
	}
}

func TestSearchContract(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r, _, _ := seedP1Env(t)

	// 全部搜索：各分区命中且隐藏内容不出现。
	rec := p1Request(r, "", http.MethodGet, "/api/search?keyword=液压", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("搜索期望 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var all struct {
		Data struct {
			Courses struct {
				Total int64 `json:"total"`
			} `json:"courses"`
			Questions struct {
				Total int64 `json:"total"`
			} `json:"questions"`
			Contents struct {
				Total int64 `json:"total"`
			} `json:"contents"`
			Topics struct {
				Total int64 `json:"total"`
			} `json:"topics"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &all); err != nil {
		t.Fatalf("解析搜索失败: %v", err)
	}
	if all.Data.Courses.Total != 1 || all.Data.Questions.Total != 1 || all.Data.Contents.Total != 1 || all.Data.Topics.Total != 1 {
		t.Fatalf("各分区应各命中 1（隐藏内容不计）: %+v", all.Data)
	}

	// 指定类型分页。
	rec = p1Request(r, "", http.MethodGet, "/api/search?keyword=液压&type=course", nil)
	var page struct {
		Data struct {
			Type  string `json:"type"`
			Total int64  `json:"total"`
			Items []struct {
				ID    int64  `json:"id"`
				Title string `json:"title"`
			} `json:"items"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &page); err != nil {
		t.Fatalf("解析类型搜索失败: %v", err)
	}
	if page.Data.Type != "course" || page.Data.Total != 1 || len(page.Data.Items) != 1 || page.Data.Items[0].Title != "液压传动维修" {
		t.Fatalf("课程搜索错误: %+v", page.Data)
	}

	// 未发布课程/草稿题/下线精选/内部帖不出现（keyword=液压 已覆盖课程与题目；用独立关键词验证下线精选）。
	rec = p1Request(r, "", http.MethodGet, "/api/search?keyword=下线&type=content", nil)
	if err := json.Unmarshal(rec.Body.Bytes(), &page); err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	if page.Data.Total != 0 {
		t.Fatalf("下线精选不应被搜索到, got %d", page.Data.Total)
	}

	// 空关键词 400；未知类型 400。
	if rec := p1Request(r, "", http.MethodGet, "/api/search?keyword=", nil); rec.Code != http.StatusBadRequest {
		t.Fatalf("空关键词期望 400, got %d", rec.Code)
	}
	if rec := p1Request(r, "", http.MethodGet, "/api/search?keyword=x&type=other", nil); rec.Code != http.StatusBadRequest {
		t.Fatalf("未知类型期望 400, got %d", rec.Code)
	}
}

func TestMaterialsContract(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r, cfg, userID := seedP1Env(t)

	token, err := security.NewSession(cfg.JWTSecretKey, time.Hour, security.CookieConfig{}).
		Issue(userID, "acct_p1", "hrwai_user")
	if err != nil {
		t.Fatalf("签发 token 失败: %v", err)
	}

	// 列表：仅已发布课程下挂章节的附件（legacy 无章节附件不含）。
	rec := p1Request(r, token, http.MethodGet, "/api/materials", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("资料列表期望 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var list struct {
		Data struct {
			Total     int64 `json:"total"`
			Materials []struct {
				FileID       int    `json:"file_id"`
				CourseName   string `json:"course_name"`
				ChapterTitle string `json:"chapter_title"`
				FileName     string `json:"file_name"`
			} `json:"materials"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil {
		t.Fatalf("解析资料列表失败: %v", err)
	}
	if list.Data.Total != 1 || len(list.Data.Materials) != 1 {
		t.Fatalf("可见资料应为 1, got %d/%d", list.Data.Total, len(list.Data.Materials))
	}
	m := list.Data.Materials[0]
	if m.CourseName != "液压传动维修" || m.ChapterTitle != "液压泵拆装" || m.FileName != "液压手册.pdf" {
		t.Fatalf("资料回填错误: %+v", m)
	}

	// /api/student/materials 别名同数据。
	rec = p1Request(r, token, http.MethodGet, "/api/student/materials", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("学员资料别名期望 200, got %d", rec.Code)
	}

	// 详情 + 下载地址；不存在 404。
	rec = p1Request(r, token, http.MethodGet, fmt.Sprintf("/api/materials/%d", m.FileID), nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("资料详情期望 200, got %d", rec.Code)
	}
	rec = p1Request(r, token, http.MethodGet, fmt.Sprintf("/api/materials/%d/download", m.FileID), nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("下载地址期望 200, got %d", rec.Code)
	}
	var dl struct {
		Data struct {
			FileURL string `json:"file_url"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &dl); err != nil {
		t.Fatalf("解析下载地址失败: %v", err)
	}
	if dl.Data.FileURL != "/static/uploads/chapters/h.pdf" {
		t.Fatalf("下载地址错误: %+v", dl.Data)
	}
	if rec := p1Request(r, token, http.MethodGet, fmt.Sprintf("/api/materials/%d", legacyFileID()), nil); rec.Code != http.StatusNotFound {
		t.Fatalf("legacy 附件（无章节）应 404, got %d", rec.Code)
	}
}

// legacyFileID 返回 legacy 附件 ID（创建序号为 2）。
func legacyFileID() int { return 2 }
