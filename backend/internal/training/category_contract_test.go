package training

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"forklift-training/internal/model"
	"forklift-training/internal/testutil"
)

func TestListTrainingCategoriesIsPublicAndReturnsOnlyActiveOptions(t *testing.T) {
	testutil.SetTestGinMode()
	db := testutil.NewMemoryDB(t)
	legacyID := 123
	categories := []model.TrainingCategory{
		{Name: "第二分类", SortOrder: 20, Status: 1, LegacyCredentialID: &legacyID},
		{Name: "历史分类", SortOrder: 5, Status: 0},
		{Name: "第一分类", SortOrder: 10, Status: 1},
	}
	if err := db.Create(&categories).Error; err != nil {
		t.Fatal(err)
	}
	router := gin.New()
	RegisterRoutes(router.Group("/api"), nil, NewService(db, zap.NewNop()))
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/catalog/categories", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("public category response = %d: %s", recorder.Code, recorder.Body)
	}
	var response struct {
		Data struct {
			Categories []map[string]any `json:"categories"`
		} `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	want := []map[string]any{
		{"id": float64(categories[2].ID), "name": categories[2].Name},
		{"id": float64(categories[0].ID), "name": categories[0].Name},
	}
	if !reflect.DeepEqual(response.Data.Categories, want) {
		t.Fatalf("public list = %#v, want %#v", response.Data.Categories, want)
	}
}
