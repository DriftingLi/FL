package training

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"forklift-training/internal/model"
	"forklift-training/internal/testutil"
)

func TestAdminCategoryMappingLifecycle(t *testing.T) {
	db := testutil.NewMemoryDB(t)
	svc := NewService(db, zap.NewNop())
	credential := model.Credential{Code: "CAT-MAP-1", Name: "叉车司机", Category: "special_operation", Status: 1}
	otherCredential := model.Credential{Code: "CAT-MAP-2", Name: "低压电工", Category: "special_operation", Status: 1}
	if err := db.Create(&credential).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&otherCredential).Error; err != nil {
		t.Fatal(err)
	}

	if _, err := svc.saveCategory(context.Background(), 0, TrainingCategoryInput{Name: "缺少映射"}); !errors.Is(err, errCategoryCredentialNeeded) {
		t.Fatalf("新建缺少映射应拒绝, got %v", err)
	}
	created, err := svc.saveCategory(context.Background(), 0, TrainingCategoryInput{Name: "维修", LegacyCredentialID: &credential.ID})
	if err != nil {
		t.Fatalf("创建分类失败: %v", err)
	}
	if created.LegacyCredentialID == nil || *created.LegacyCredentialID != credential.ID {
		t.Fatalf("创建响应应回显映射, got %+v", created)
	}

	legacy := model.TrainingCategory{Name: "历史空映射", Status: 1}
	if err := db.Create(&legacy).Error; err != nil {
		t.Fatal(err)
	}
	renamed, err := svc.saveCategory(context.Background(), legacy.ID, TrainingCategoryInput{Name: "历史空映射（改名）"})
	if err != nil {
		t.Fatalf("历史空映射分类应允许只改名: %v", err)
	}
	if renamed.LegacyCredentialID != nil {
		t.Fatalf("未提供映射的更新应保留空映射, got %+v", renamed)
	}
	updated, err := svc.saveCategory(context.Background(), legacy.ID, TrainingCategoryInput{Name: legacy.Name, LegacyCredentialID: &otherCredential.ID})
	if err != nil {
		t.Fatalf("补录历史映射失败: %v", err)
	}
	if updated.LegacyCredentialID == nil || *updated.LegacyCredentialID != otherCredential.ID {
		t.Fatalf("更新响应应回显新映射, got %+v", updated)
	}

	if _, err := svc.saveCategory(context.Background(), 0, TrainingCategoryInput{Name: "重复映射", LegacyCredentialID: &credential.ID}); !errors.Is(err, errCategoryCredentialUsed) {
		t.Fatalf("重复证件映射应拒绝, got %v", err)
	}
	missingID := 99999
	if _, err := svc.saveCategory(context.Background(), 0, TrainingCategoryInput{Name: "不存在证件", LegacyCredentialID: &missingID}); !errors.Is(err, errCategoryCredentialNotFound) {
		t.Fatalf("不存在证件应拒绝, got %v", err)
	}

	list, err := svc.listAdminCategories(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	byID := make(map[int]AdminTrainingCategoryDTO, len(list.Categories))
	for _, item := range list.Categories {
		byID[item.ID] = item
	}
	if got := byID[created.ID]; got.LegacyCredentialID == nil || *got.LegacyCredentialID != credential.ID {
		t.Fatalf("管理列表应返回映射, got %+v", got)
	}
	if got := byID[legacy.ID]; got.LegacyCredentialID == nil || *got.LegacyCredentialID != otherCredential.ID {
		t.Fatalf("管理列表应返回补录后的映射, got %+v", got)
	}
}

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
