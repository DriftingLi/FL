// #1132 契约测试：章节收藏的可见性**跟随所属课程**（与搜索的章节分区同一谓词）。
//
// 口径来源：course_mount_scope.go 的文件头自述把「收藏目标校验」列为该 SQL 形态的消费方，
// 而 favorite 的章节支此前只校验「章节存在」⇒ 自述与实现不一致。本票补齐：
// 章节收藏要求所属课程**已发布（status = 1）且已挂载**（specialty_id / level_id 均非空）。
//
// 读面（favoriteTargetsMeta / List）保持「写时校验、读到快照」的既有形状（course 支即如此），
// 不在本测试的断言面内。
package api

import (
	"bytes"
	"encoding/json"
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

func TestFavoriteChapterVisibilityContract(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := testutil.NewMemoryDB(t)

	ptr := func(v int) *int { return &v }
	spec := model.Specialty{Code: "vis", Name: "可见性", SortOrder: 1, Status: 1}
	if err := db.Create(&spec).Error; err != nil {
		t.Fatalf("创建方向失败: %v", err)
	}
	lv := model.CourseLevel{Code: "vis-lv", Name: "入门", SortOrder: 1, Status: 1}
	if err := db.Create(&lv).Error; err != nil {
		t.Fatalf("创建等级失败: %v", err)
	}

	// 可见课程：已发布 + 已挂载。
	visible := model.Course{Name: "可见课程", Status: 1, SpecialtyID: ptr(spec.SpecialtyID), LevelID: ptr(lv.LevelID), CreatedAt: testutil.Now()}
	if err := db.Create(&visible).Error; err != nil {
		t.Fatalf("创建可见课程失败: %v", err)
	}
	// 未发布课程：已挂载但 status = 0（Course.Status 带 gorm default:1，零值会被默认值覆盖，需显式回写）。
	unpublished := model.Course{Name: "未发布课程", Status: 0, SpecialtyID: ptr(spec.SpecialtyID), LevelID: ptr(lv.LevelID), CreatedAt: testutil.Now()}
	if err := db.Create(&unpublished).Error; err != nil {
		t.Fatalf("创建未发布课程失败: %v", err)
	}
	if err := db.Model(&model.Course{}).Where("course_id = ?", unpublished.CourseID).Update("status", 0).Error; err != nil {
		t.Fatalf("置未发布状态失败: %v", err)
	}
	// 未挂载课程：已发布但缺专业方向 / 等级（挂载不变式不成立）。
	unmounted := model.Course{Name: "未挂载课程", Status: 1, CreatedAt: testutil.Now()}
	if err := db.Create(&unmounted).Error; err != nil {
		t.Fatalf("创建未挂载课程失败: %v", err)
	}

	visibleCh := model.Chapter{CourseID: visible.CourseID, Title: "可见章节", OrderNum: 1, CreatedAt: testutil.Now()}
	unpublishedCh := model.Chapter{CourseID: unpublished.CourseID, Title: "未发布课程的章节", OrderNum: 1, CreatedAt: testutil.Now()}
	unmountedCh := model.Chapter{CourseID: unmounted.CourseID, Title: "未挂载课程的章节", OrderNum: 1, CreatedAt: testutil.Now()}
	for _, ch := range []*model.Chapter{&visibleCh, &unpublishedCh, &unmountedCh} {
		if err := db.Create(ch).Error; err != nil {
			t.Fatalf("创建章节失败: %v", err)
		}
	}

	user := model.HrwaiUser{Account: "acct_vis", Phone: "13800000077", Username: "可见性", Status: 1, CreatedAt: testutil.Now()}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("创建用户失败: %v", err)
	}

	cfg := &config.Config{JWTSecretKey: "contract-test-secret", AuthCookie: config.AuthCookieConfig{Name: "hrwai_token"}}
	r := gin.New()
	apiGroup := r.Group("/api")
	deps := newContractDeps(t, db, cfg)
	RegisterFavoriteRoutes(apiGroup, deps.RouterDeps(), deps.FavoriteSvc)

	token, err := security.NewSession(cfg.JWTSecretKey, time.Hour, security.CookieConfig{}).Issue(int(user.ID), user.Account, "hrwai_user")
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

	// 可见课程的章节：可收藏 201，且响应带所属课程 ID（章节落点 chapter-view 需要它）。
	rec := do(http.MethodPost, "/api/favorites", map[string]any{"target_type": "chapter", "target_id": visibleCh.ChapterID})
	if rec.Code != http.StatusCreated {
		t.Fatalf("可见课程的章节应可收藏 201, got %d: %s", rec.Code, rec.Body.String())
	}
	var added struct {
		Data struct {
			CourseID int `json:"course_id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &added); err != nil {
		t.Fatalf("解析收藏响应失败: %v", err)
	}
	if added.Data.CourseID != visible.CourseID {
		t.Fatalf("章节收藏应带所属课程 ID %d, got %+v", visible.CourseID, added.Data)
	}

	// 不可见章节：未发布课程的章节、未挂载课程的章节 —— 两者都必须被拒（与搜索章节分区同一谓词）。
	for _, tc := range []struct {
		name string
		id   int
	}{
		{"未发布课程的章节", unpublishedCh.ChapterID},
		{"未挂载课程的章节", unmountedCh.ChapterID},
	} {
		rec := do(http.MethodPost, "/api/favorites", map[string]any{"target_type": "chapter", "target_id": tc.id})
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("%s 应被拒 400, got %d: %s", tc.name, rec.Code, rec.Body.String())
		}
	}

	// 反例守卫：被拒的两次不得留下收藏行；列表只应出现可见章节这一条。
	rec = do(http.MethodGet, "/api/favorites?target_type=chapter", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("章节收藏列表期望 200, got %d", rec.Code)
	}
	var list struct {
		Data struct {
			Total     int64 `json:"total"`
			Favorites []struct {
				TargetID int `json:"target_id"`
				CourseID int `json:"course_id"`
			} `json:"favorites"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil {
		t.Fatalf("解析章节收藏列表失败: %v", err)
	}
	if list.Data.Total != 1 || len(list.Data.Favorites) != 1 || list.Data.Favorites[0].TargetID != visibleCh.ChapterID {
		t.Fatalf("章节收藏列表应只含可见章节一条, got %+v", list.Data)
	}

	// 不变量（#1132 复审）：章节收藏项**恒带有效课程 ID**，前端 `course_id > 0` 只能是纵深防御、
	// 不可能是常态分支 —— 依据是结构而非假设：`chapter.course_id` 有外键 `chapter_course_id_fkey`
	// （非 0、非孤立），且 List 只回 `meta.Found` 的行（章节被删则整行不出现）。
	// 钉住它：任何让章节项带 `course_id = 0`（或串到别的课程）的实现改动都必须在这里变红。
	var owningCourseID int
	if err := db.Model(&model.Chapter{}).Where("chapter_id = ?", visibleCh.ChapterID).
		Pluck("course_id", &owningCourseID).Error; err != nil {
		t.Fatalf("读取章节所属课程失败: %v", err)
	}
	if owningCourseID <= 0 {
		t.Fatalf("夹具失效：章节应有所属课程, got %d", owningCourseID)
	}
	if got := list.Data.Favorites[0].CourseID; got != owningCourseID {
		t.Fatalf("章节收藏项应恒带所属课程 ID %d, got %d", owningCourseID, got)
	}
	if added.Data.CourseID != owningCourseID {
		t.Fatalf("Add 路径应与 List 路径同口径（课程 ID %d）, got %d", owningCourseID, added.Data.CourseID)
	}
}
