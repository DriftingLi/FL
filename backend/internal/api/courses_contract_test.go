// Ticket 1a（issue #51）API 契约测试：
//   - /api/courses 不再消费 category 参数（传入被忽略，课程照常返回）
//   - 响应课程不含 category，包含 chapter_count 与 prerequisite_course_ids
package api

import (
	"encoding/json"
	"net/http"
	"testing"

	"go.uber.org/zap"

	"github.com/gin-gonic/gin"

	"forklift-training/internal/config"
	"forklift-training/internal/model"
	"forklift-training/internal/testutil"
)

func TestCoursesListCategoryParamRetired(t *testing.T) {
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
	course := model.Course{Name: "契约课程", Status: 1,
		SpecialtyID: ptr(spec.SpecialtyID), LevelID: ptr(lv.LevelID), CreatedAt: testutil.Now()}
	if err := db.Create(&course).Error; err != nil {
		t.Fatalf("创建课程失败: %v", err)
	}
	ch := model.Chapter{CourseID: course.CourseID, Title: "第一章", OrderNum: 1, CreatedAt: testutil.Now()}
	if err := db.Create(&ch).Error; err != nil {
		t.Fatalf("创建章节失败: %v", err)
	}

	r := gin.New()
	api := r.Group("/api")
	RegisterCoursesRoutes(api, &config.Config{}, db, nil, zap.NewNop())

	// 传入已退役的 category 参数：应被忽略，课程仍返回
	rec := performRequest(r, "GET", "/api/courses?category=CATEGORY_01")
	if rec.Code != http.StatusOK {
		t.Fatalf("期望 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var body struct {
		Data struct {
			Courses []map[string]any `json:"courses"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("解析响应失败: %v", err)
	}
	if len(body.Data.Courses) == 0 {
		t.Fatal("应返回课程, got 0")
	}
	var item map[string]any
	for _, c := range body.Data.Courses {
		if c["name"] == "契约课程" {
			item = c
		}
	}
	if item == nil {
		t.Fatalf("未找到契约课程: %+v", body.Data.Courses)
	}
	if _, ok := item["category"]; ok {
		t.Fatal("响应课程不应包含 category")
	}
	if item["chapter_count"] != float64(1) {
		t.Fatalf("chapter_count 应为 1, got %v", item["chapter_count"])
	}
	if ids, ok := item["prerequisite_course_ids"].([]any); !ok || len(ids) != 0 {
		t.Fatalf("prerequisite_course_ids 应为空数组, got %v", item["prerequisite_course_ids"])
	}
}
