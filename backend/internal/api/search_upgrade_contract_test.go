// ADR-0049 / #982 契约：分区扩展与结果形状（章节分区、命中片段、命中位置、落点参数），
// 以及零结果词运营面的权限口径（管理员可读、学员 403）。
package api

import (
	"net/http"
	"testing"

	"forklift-training/internal/model"
	"forklift-training/internal/testutil"
)

func TestSearchUpgradeContract(t *testing.T) {
	r, cfg, db := newSlice6Env(t)
	student := testutil.SeedStudent(t, db, "search_upgrade_stu", "hash")

	spID, lvID := 1, 1
	course := model.Course{Name: "液压系统课程", Status: 1, SpecialtyID: &spID, LevelID: &lvID, CreatedAt: testutil.Now()}
	if err := db.Create(&course).Error; err != nil {
		t.Fatalf("建课失败: %v", err)
	}
	ch := model.Chapter{CourseID: course.CourseID, Title: "液压泵拆装工艺", Content: "本节讲液压泵", CreatedAt: testutil.Now()}
	if err := db.Create(&ch).Error; err != nil {
		t.Fatalf("建章节失败: %v", err)
	}

	// 聚合路径：新增 chapters 分区，且条目形状带命中片段 / 命中位置 / 落点参数
	rec := catalogRequest(t, r, "", http.MethodGet, "/api/search?keyword=%E6%B6%B2%E5%8E%8B", "")
	m := slice6AssertKeys(t, rec, 200, "chapters", "contents", "courses", "keyword", "questions", "topics")
	chapters, _ := m["chapters"].(map[string]any)
	items, _ := chapters["items"].([]any)
	if len(items) != 1 {
		t.Fatalf("章节分区应有 1 条, got %v", chapters)
	}
	item, _ := items[0].(map[string]any)
	assertDictKeys(t, item, []string{"cover", "hit_field", "id", "parent_id", "snippet", "summary", "title", "type"})
	if item["hit_field"] != "title" {
		t.Fatalf("章节命中位置应为 title, got %v", item["hit_field"])
	}
	if item["parent_id"] == nil || item["parent_id"].(float64) <= 0 {
		t.Fatalf("章节结果必须带所属课程 ID（落点参数）, got %v", item["parent_id"])
	}

	// 指定 type=chapter 的分页形状
	rec = catalogRequest(t, r, "", http.MethodGet, "/api/search?keyword=%E6%B6%B2%E5%8E%8B&type=chapter", "")
	slice6AssertKeys(t, rec, 200, "items", "keyword", "page", "pages", "total", "type")

	// 零结果词运营面：管理员可读、学员 403
	adminToken := slice6Token(t, cfg, 1, "contract_admin", "admin")
	rec = catalogRequest(t, r, adminToken, http.MethodGet, "/api/admin/search-facts/zero-results", "")
	unpackData(t, rec, 200) // data 是**数组**（零结果词列表），不是对象
	stuToken := slice6Token(t, cfg, student.ID, student.Account, "hrwai_user")
	rec = catalogRequest(t, r, stuToken, http.MethodGet, "/api/admin/search-facts/zero-results", "")
	if rec.Code != http.StatusForbidden {
		t.Fatalf("学员读零结果词应 403, got %d %s", rec.Code, rec.Body.String())
	}
}
