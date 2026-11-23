package course

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"forklift-training/internal/model"
	"forklift-training/internal/security"
	"forklift-training/internal/testutil"
)

func TestPublicCourseDetailDoesNotExposeLessonContent(t *testing.T) {
	testutil.SetTestGinMode()
	db := testutil.NewMemoryDB(t)
	category := model.TrainingCategory{Name: "维修", Status: 1}
	if err := db.Create(&category).Error; err != nil {
		t.Fatal(err)
	}
	price := 12
	courses := []model.Course{
		{Name: "已发布课程", CategoryID: &category.ID, PointsPrice: &price, Status: 1, IsHot: true},
		{Name: "未分类课程", Status: 1},
		{Name: "未发布课程", CategoryID: &category.ID, Status: 1},
	}
	if err := db.Create(&courses).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&courses[2]).Update("status", 0).Error; err != nil {
		t.Fatal(err)
	}
	chapters := []model.Chapter{
		{CourseID: courses[0].CourseID, Title: "先读", OrderNum: 1, Content: "私人正文", ContentType: "text", FileURL: "/private/working"},
		{CourseID: courses[0].CourseID, Title: "后读", OrderNum: 2},
		{CourseID: courses[0].CourseID, Title: "在审章节", OrderNum: 0},
	}
	if err := db.Create(&chapters).Error; err != nil {
		t.Fatal(err)
	}
	slots := []model.ChapterContentSlot{
		{ChapterID: chapters[1].ChapterID, Slot: model.ChapterReviewPublished, ContentType: "text"},
		{ChapterID: chapters[0].ChapterID, Slot: model.ChapterReviewPublished, ContentType: "video", FileURL: "/private/published"},
		{ChapterID: chapters[2].ChapterID, Slot: model.ChapterReviewPending, ContentType: "text"},
	}
	if err := db.Create(&slots).Error; err != nil {
		t.Fatal(err)
	}

	router := gin.New()
	session := security.NewSession("public-course-test", time.Hour, security.CookieConfig{})
	RegisterRoutes(router.Group("/api"), session, nil, NewService(db, nil, zap.NewNop()))
	get := func(path string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
		return w
	}
	result := get(fmt.Sprintf("/api/courses/%d", courses[0].CourseID))
	if result.Code != http.StatusOK {
		t.Fatalf("public course: %d %s", result.Code, result.Body)
	}
	var envelope struct {
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(result.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	var detail PublicCourseDetailDTO
	if err := json.Unmarshal(envelope.Data, &detail); err != nil {
		t.Fatal(err)
	}
	if detail.Category.ID != category.ID || detail.PointsPrice == nil || *detail.PointsPrice != price ||
		detail.ChapterCount != 2 || len(detail.Chapters) != 2 ||
		detail.Chapters[0].Title != "先读" || detail.Chapters[0].ContentType != "video" || detail.Chapters[1].Title != "后读" {
		t.Fatalf("unexpected public detail: %s", envelope.Data)
	}
	for _, key := range []string{`"content":`, `"file_url":`, `"entitled":`, `"progress":`, "在审章节", "/private/"} {
		if strings.Contains(string(envelope.Data), key) {
			t.Errorf("public detail contains %s: %s", key, envelope.Data)
		}
	}
	for _, id := range []int{courses[1].CourseID, courses[2].CourseID, 99999} {
		if response := get(fmt.Sprintf("/api/courses/%d", id)); response.Code != http.StatusNotFound {
			t.Errorf("course %d should not be visible: %d %s", id, response.Code, response.Body)
		}
	}
	if response := get(fmt.Sprintf("/api/course/%d/chapter/%d", courses[0].CourseID, chapters[0].ChapterID)); response.Code != http.StatusUnauthorized {
		t.Errorf("anonymous chapter should be denied: %d %s", response.Code, response.Body)
	}
	for _, check := range []struct {
		path string
		want int
	}{
		{"/api/catalog/courses", 1},
		{fmt.Sprintf("/api/catalog/courses?category_id=%d", category.ID), 1},
		{"/api/catalog/courses?category_id=99999", 0},
		{"/api/catalog/courses?filter=hot", 1},
		{"/api/catalog/courses?filter=featured", 0},
	} {
		result := get(check.path)
		var page struct {
			Data PublicCoursePageDTO `json:"data"`
		}
		if err := json.Unmarshal(result.Body.Bytes(), &page); err != nil ||
			result.Code != http.StatusOK || page.Data.Total != int64(check.want) || len(page.Data.Courses) != check.want {
			t.Fatalf("%s: %d %s (%v)", check.path, result.Code, result.Body, err)
		}
		if check.want == 1 && (page.Data.Courses[0].Category.ID != category.ID || page.Data.Courses[0].Category.Name != category.Name) {
			t.Errorf("public list lost category projection: %s", result.Body)
		}
	}
}
