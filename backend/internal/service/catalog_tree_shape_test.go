package service

import (
	"encoding/json"
	"testing"
)

// TestCatalogTreeDTOShapeLock 目录树契约冻结：根/方向/等级节点的 key 集合与旧 map 投影一致。
func TestCatalogTreeDTOShapeLock(t *testing.T) {
	assertShapeLock(t, CatalogTreeDTO{Specialties: nil}, "specialties")

	leaf := CatalogLevelNode{
		Code: "beginner", Courses: []CourseDTO{}, LevelID: 1, Name: "入门",
	}
	assertShapeLock(t, leaf,
		"code", "created_at", "courses", "description", "level_id", "name", "sort_order", "status",
	)

	spec := CatalogSpecialtyNode{
		Code: "operation", Levels: []CatalogLevelNode{leaf}, SpecialtyID: 1, Name: "操作",
	}
	assertShapeLock(t, spec,
		"code", "created_at", "description", "levels", "name", "sort_order", "specialty_id", "status",
	)
}

// TestCatalogTreeDTO_BytesMatchLegacy 字节级契约：与旧 map 投影（json 按键排序）逐字一致。
func TestCatalogTreeDTO_BytesMatchLegacy(t *testing.T) {
	now := "2026-08-01T09:00:00.000000"
	spec := CatalogSpecialtyNode{
		Code:        "operation",
		CreatedAt:   now,
		Description: "操作方向",
		Levels: []CatalogLevelNode{{
			Code:        "beginner",
			CreatedAt:   now,
			Courses:     []CourseDTO{},
			Description: "入门",
			LevelID:     1,
			Name:        "入门",
			SortOrder:   0,
			Status:      1,
		}},
		Name:        "操作",
		SortOrder:   0,
		SpecialtyID: 1,
		Status:      1,
	}
	tree := CatalogTreeDTO{Specialties: []CatalogSpecialtyNode{spec}}

	legacy := map[string]any{
		"specialties": []map[string]any{{
			"code":         "operation",
			"created_at":   now,
			"description":  "操作方向",
			"name":         "操作",
			"sort_order":   0,
			"specialty_id": 1,
			"status":       int16(1),
			"levels": []map[string]any{{
				"code":        "beginner",
				"created_at":  now,
				"courses":     []CourseDTO{},
				"description": "入门",
				"level_id":    1,
				"name":        "入门",
				"sort_order":  0,
				"status":      int16(1),
			}},
		}},
	}

	got, _ := json.Marshal(tree)
	want, _ := json.Marshal(legacy)
	if string(got) != string(want) {
		t.Errorf("目录树契约漂移\n got: %s\nwant: %s", got, want)
	}
}
