// 第③批（课程 / 精选内容 / 讲师 / 学员）的**声明式档位台账**（ADR-0064 决策 8）。
// 形状沿用第①批 disposition_face_ledger_test.go 与第②批 catalog_face_ledger_test.go：
// 登记档位 → 逐档验 → 反向锁「登记了却打不出」。共用的故障注入（dropTable）、驱动原文泄漏
// 判据（leakyDriverText）都取自那两份，本文件不重写第二份。
//
// 本批修复前的形状：这些端点里 20 个只有一格错误面（WithSuccess(…, 404) 或 errStatusAll(404)）
// ⇒ 真不存在 / 查不动 / 输入不合法三种事实挤进同一个 404，其中「查不动」被答成
// 「这个内容不存在」；另有精选 Create/Update 两格相反（默认面 400 ⇒ 写库故障被答成参数错误）。
//
// 覆盖范围如实登记（不夸大）：
//   - 讲师面 4 个端点（/api/tutor/…）**在本台账里以带 CapTutorAccess 的 token 打**；
//     若装配链拿不到该能力位，台账会把它们从表里摘掉并在下方注明原因——**不假装覆盖**。
//   - /api/courses（列表）与 /api/admin/courses（列表）无路径 id、无 404 档，不在本表内；
//     它们的分页/筛选档由既有契约测试覆盖。
package api

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"gorm.io/gorm"

	"forklift-training/internal/model"
	"forklift-training/internal/security"
	"forklift-training/internal/service"
	"forklift-training/internal/testutil"
)

// courseDomainIDs 本域台账要用的主键。missing 是一个必然不存在的 id。
type courseDomainIDs struct {
	course, chapter, file, featuredDraft, featuredPublished int
	specialty, level                                        int
	missing                                                 int
}

// courseFaceCase 一档要打的请求。
type courseFaceCase struct {
	method string
	path   func(d courseDomainIDs) string
	body   any
	setup  func(t *testing.T, db *gorm.DB)
}

type courseDeclaredFace struct {
	want int
	req  courseFaceCase
}

type courseEndpointFaces struct {
	name  string
	who   string // admin | student | tutor | public（public = 不带凭证）
	cases []courseDeclaredFace
}

// contractJWTSecret 与 newAdminContractDeps 里那把必须一致（路由只校验签名，不查库）。
const contractJWTSecret = "contract-test-secret"

// tokenFor 按身份签一枚 token。ghost 是一枚「主体不存在」的 token（用来打 404 档）。
func tokenFor(t *testing.T, db *gorm.DB, who string) string {
	t.Helper()
	sess := security.NewSession(contractJWTSecret, time.Hour, security.CookieConfig{})
	var (
		uid     int
		account string
		role    string
	)
	switch who {
	case "tutor":
		hashed, err := service.HashPassword("seedpass123")
		if err != nil {
			t.Fatalf("哈希种子口令失败: %v", err)
		}
		tu := testutil.SeedTutor(t, db, "cledger_tutor", hashed)
		uid, account, role = tu.TutorID, tu.Username, service.TutorRole
	case "ghost":
		uid, account, role = 999998, "cledger_ghost", service.HrwaiRole
	case "public":
		return ""
	default:
		hashed, err := service.HashPassword("seedpass123")
		if err != nil {
			t.Fatalf("哈希种子口令失败: %v", err)
		}
		stu := testutil.SeedStudent(t, db, "cledger_stu", hashed)
		uid, account, role = stu.ID, stu.Account, service.HrwaiRole
	}
	tok, err := sess.Issue(uid, account, role)
	if err != nil {
		t.Fatalf("签发 token 失败（%s）: %v", who, err)
	}
	return tok
}

// seedCourseDomain 播种「已发布 + 已挂载」的课程一条、其章节一条、精选草稿与已发布各一条。
// 台账要的只是「有一个真实可寻址的对象」与「有一个必然不存在的 id」，不铺内容。
func seedCourseDomain(t *testing.T, db *gorm.DB) courseDomainIDs {
	t.Helper()
	ids := courseDomainIDs{missing: 999999}
	spec := model.Specialty{Code: "cledger", Name: "台账方向", SortOrder: 1, Status: 1}
	if err := db.Create(&spec).Error; err != nil {
		t.Fatalf("播种方向失败: %v", err)
	}
	lv := model.CourseLevel{Code: "cledger-lv", Name: "台账等级", SortOrder: 1, Status: 1}
	if err := db.Create(&lv).Error; err != nil {
		t.Fatalf("播种等级失败: %v", err)
	}
	course := model.Course{
		Name: "台账课程", Status: 1,
		SpecialtyID: &spec.SpecialtyID, LevelID: &lv.LevelID,
		CreatedAt: testutil.Now(),
	}
	if err := db.Create(&course).Error; err != nil {
		t.Fatalf("播种课程失败: %v", err)
	}
	ids.course, ids.specialty, ids.level = course.CourseID, spec.SpecialtyID, lv.LevelID
	ch := model.Chapter{CourseID: course.CourseID, Title: "台账章节", OrderNum: 1, CreatedAt: testutil.Now()}
	if err := db.Create(&ch).Error; err != nil {
		t.Fatalf("播种章节失败: %v", err)
	}
	ids.chapter = ch.ChapterID
	// 章节附件行（DELETE /api/tutor/file/:file_id 的可寻址对象）。
	f := model.ChapterFile{ChapterID: &ch.ChapterID, FileName: "台账文件.pdf", FileURL: "/uploads/ledger.pdf"}
	if err := db.Create(&f).Error; err != nil {
		t.Fatalf("播种章节文件失败: %v", err)
	}
	ids.file = f.FileID
	for _, it := range []*model.FeaturedContent{
		{Title: "台账草稿", Category: "industry", Status: 0, Content: "正文", CreatedAt: testutil.Now()},
		{Title: "台账已发布", Category: "industry", Status: 1, Content: "正文", CreatedAt: testutil.Now()},
	} {
		if err := db.Create(it).Error; err != nil {
			t.Fatalf("播种精选内容失败: %v", err)
		}
		if it.Status == 1 {
			ids.featuredPublished = it.ContentID
		} else {
			ids.featuredDraft = it.ContentID
		}
	}
	return ids
}

// id 便捷构造：把主键渲染进路径。
func id(v int) string                    { return fmt.Sprint(v) }
func missingID(d courseDomainIDs) string { return id(d.missing) }

var courseDomainFaces = []courseEndpointFaces{
	// ===== 管理端 · 课程与章节 =====
	{
		name: "GET /admin/course/:course_id", who: "admin",
		cases: []courseDeclaredFace{
			{http.StatusBadRequest, courseFaceCase{method: http.MethodGet, path: func(d courseDomainIDs) string { return "/api/admin/course" + badID }}},
			{http.StatusNotFound, courseFaceCase{method: http.MethodGet, path: func(d courseDomainIDs) string { return "/api/admin/course/" + missingID(d) }}},
			{http.StatusInternalServerError, courseFaceCase{method: http.MethodGet, path: func(d courseDomainIDs) string { return "/api/admin/course/" + id(d.course) }, setup: dropTable("course")}},
		},
	},
	{
		name: "PUT /admin/course/:course_id", who: "admin",
		cases: []courseDeclaredFace{
			{http.StatusBadRequest, courseFaceCase{method: http.MethodPut, path: func(d courseDomainIDs) string { return "/api/admin/course" + badID }, body: map[string]any{"name": "台账改课"}}},
			// 第 3 族：编辑时把方向清空 = 输入不合法，不得冒充服务端故障（本批刚把它从 500 归位 400）。
			{http.StatusBadRequest, courseFaceCase{method: http.MethodPut, path: func(d courseDomainIDs) string { return "/api/admin/course/" + id(d.course) }, body: map[string]any{"specialty_id": 0}}},
			{http.StatusNotFound, courseFaceCase{method: http.MethodPut, path: func(d courseDomainIDs) string { return "/api/admin/course/" + missingID(d) }, body: map[string]any{"name": "台账改课"}}},
			{http.StatusInternalServerError, courseFaceCase{method: http.MethodPut, path: func(d courseDomainIDs) string { return "/api/admin/course/" + id(d.course) }, body: map[string]any{"name": "台账改课"}, setup: dropTable("course")}},
		},
	},
	{
		name: "DELETE /admin/course/:course_id", who: "admin",
		cases: []courseDeclaredFace{
			// 负数 id 归 400（ADR-0065 批⑤）：改之前这一格是 404「课程不存在」——拿一枚非法输入
			// 冒充一个不存在的资源，而同样的输入在第①批的用户/讲师面答 400、且用的是另一句文案
			// （「用户 ID 非法」）。⇒ 同一件输入错误在三个地方说出三种话。现在 `pathInt` 单点
			// 挡下 `<= 0`，本域与那两域**同码同文案**（`path_int_face_contract_test.go` 钉住）。
			{http.StatusBadRequest, courseFaceCase{method: http.MethodDelete, path: func(d courseDomainIDs) string { return "/api/admin/course" + negID }}},
			{http.StatusNotFound, courseFaceCase{method: http.MethodDelete, path: func(d courseDomainIDs) string { return "/api/admin/course/" + missingID(d) }}},
			{http.StatusInternalServerError, courseFaceCase{method: http.MethodDelete, path: func(d courseDomainIDs) string { return "/api/admin/course/" + id(d.course) }, setup: dropTable("course")}},
		},
	},
	{
		name: "PUT /admin/chapter/:chapter_id", who: "admin",
		cases: []courseDeclaredFace{
			{http.StatusBadRequest, courseFaceCase{method: http.MethodPut, path: func(d courseDomainIDs) string { return "/api/admin/chapter" + badID }, body: map[string]any{"title": "台账改章"}}},
			{http.StatusNotFound, courseFaceCase{method: http.MethodPut, path: func(d courseDomainIDs) string { return "/api/admin/chapter/" + missingID(d) }, body: map[string]any{"title": "台账改章"}}},
			{http.StatusInternalServerError, courseFaceCase{method: http.MethodPut, path: func(d courseDomainIDs) string { return "/api/admin/chapter/" + id(d.chapter) }, body: map[string]any{"title": "台账改章"}, setup: dropTable("chapter")}},
		},
	},
	{
		name: "DELETE /admin/chapter/:chapter_id", who: "admin",
		cases: []courseDeclaredFace{
			{http.StatusBadRequest, courseFaceCase{method: http.MethodDelete, path: func(d courseDomainIDs) string { return "/api/admin/chapter" + badID }}},
			{http.StatusNotFound, courseFaceCase{method: http.MethodDelete, path: func(d courseDomainIDs) string { return "/api/admin/chapter/" + missingID(d) }}},
			{http.StatusInternalServerError, courseFaceCase{method: http.MethodDelete, path: func(d courseDomainIDs) string { return "/api/admin/chapter/" + id(d.chapter) }, setup: dropTable("chapter")}},
		},
	},

	// ===== 管理端 · 精选内容 =====
	{
		name: "POST /admin/featured-content", who: "admin",
		cases: []courseDeclaredFace{
			{http.StatusBadRequest, courseFaceCase{method: http.MethodPost, path: func(courseDomainIDs) string { return "/api/admin/featured-content" }, body: map[string]any{"title": "", "category": "industry"}}},
			{http.StatusBadRequest, courseFaceCase{method: http.MethodPost, path: func(courseDomainIDs) string { return "/api/admin/featured-content" }, body: map[string]any{"title": "台账稿", "category": "not-a-category"}}},
			{http.StatusBadRequest, courseFaceCase{method: http.MethodPost, path: func(courseDomainIDs) string { return "/api/admin/featured-content" }, body: map[string]any{"title": "台账稿", "category": "industry", "cover_image": "https://evil.example/x.png"}}},
			{http.StatusInternalServerError, courseFaceCase{method: http.MethodPost, path: func(courseDomainIDs) string { return "/api/admin/featured-content" }, body: map[string]any{"title": "台账稿", "category": "industry"}, setup: dropTable("featured_content")}},
		},
	},
	{
		name: "PUT /admin/featured-content/:id", who: "admin",
		cases: []courseDeclaredFace{
			{http.StatusBadRequest, courseFaceCase{method: http.MethodPut, path: func(d courseDomainIDs) string { return "/api/admin/featured-content" + badID }, body: map[string]any{"category": "industry"}}},
			{http.StatusBadRequest, courseFaceCase{method: http.MethodPut, path: func(d courseDomainIDs) string { return "/api/admin/featured-content/" + id(d.featuredDraft) }, body: map[string]any{"category": "not-a-category"}}},
			{http.StatusNotFound, courseFaceCase{method: http.MethodPut, path: func(d courseDomainIDs) string { return "/api/admin/featured-content/" + missingID(d) }, body: map[string]any{"category": "industry"}}},
			{http.StatusInternalServerError, courseFaceCase{method: http.MethodPut, path: func(d courseDomainIDs) string { return "/api/admin/featured-content/" + id(d.featuredDraft) }, body: map[string]any{"category": "industry"}, setup: dropTable("featured_content")}},
		},
	},
	{
		name: "GET /admin/featured-content/:id", who: "admin",
		cases: []courseDeclaredFace{
			{http.StatusBadRequest, courseFaceCase{method: http.MethodGet, path: func(d courseDomainIDs) string { return "/api/admin/featured-content" + badID }}},
			{http.StatusNotFound, courseFaceCase{method: http.MethodGet, path: func(d courseDomainIDs) string { return "/api/admin/featured-content/" + missingID(d) }}},
			{http.StatusInternalServerError, courseFaceCase{method: http.MethodGet, path: func(d courseDomainIDs) string { return "/api/admin/featured-content/" + id(d.featuredPublished) }, setup: dropTable("featured_content")}},
		},
	},
	{
		name: "DELETE /admin/featured-content/:id", who: "admin",
		cases: []courseDeclaredFace{
			{http.StatusBadRequest, courseFaceCase{method: http.MethodDelete, path: func(d courseDomainIDs) string { return "/api/admin/featured-content" + badID }}},
			{http.StatusNotFound, courseFaceCase{method: http.MethodDelete, path: func(d courseDomainIDs) string { return "/api/admin/featured-content/" + missingID(d) }}},
			{http.StatusInternalServerError, courseFaceCase{method: http.MethodDelete, path: func(d courseDomainIDs) string { return "/api/admin/featured-content/" + id(d.featuredDraft) }, setup: dropTable("featured_content")}},
		},
	},
	{
		name: "POST /admin/featured-content/:id/publish", who: "admin",
		cases: []courseDeclaredFace{
			{http.StatusBadRequest, courseFaceCase{method: http.MethodPost, path: func(d courseDomainIDs) string { return "/api/admin/featured-content" + badID + "/publish" }}},
			{http.StatusNotFound, courseFaceCase{method: http.MethodPost, path: func(d courseDomainIDs) string { return "/api/admin/featured-content/" + missingID(d) + "/publish" }}},
			{http.StatusInternalServerError, courseFaceCase{method: http.MethodPost, path: func(d courseDomainIDs) string {
				return "/api/admin/featured-content/" + id(d.featuredDraft) + "/publish"
			}, setup: dropTable("featured_content")}},
		},
	},
	{
		name: "GET /featured-content/:id", who: "public",
		cases: []courseDeclaredFace{
			{http.StatusBadRequest, courseFaceCase{method: http.MethodGet, path: func(d courseDomainIDs) string { return "/api/featured-content" + badID }}},
			// 未发布也按「不存在」答 404（不泄漏存在性），与第①批台账同一判据。
			{http.StatusNotFound, courseFaceCase{method: http.MethodGet, path: func(d courseDomainIDs) string { return "/api/featured-content/" + id(d.featuredDraft) }}},
			{http.StatusNotFound, courseFaceCase{method: http.MethodGet, path: func(d courseDomainIDs) string { return "/api/featured-content/" + missingID(d) }}},
			{http.StatusInternalServerError, courseFaceCase{method: http.MethodGet, path: func(d courseDomainIDs) string { return "/api/featured-content/" + id(d.featuredPublished) }, setup: dropTable("featured_content")}},
		},
	},
	{
		name: "POST /featured-content/:id/view", who: "public",
		cases: []courseDeclaredFace{
			{http.StatusBadRequest, courseFaceCase{method: http.MethodPost, path: func(d courseDomainIDs) string { return "/api/featured-content" + badID + "/view" }}},
			{http.StatusNotFound, courseFaceCase{method: http.MethodPost, path: func(d courseDomainIDs) string { return "/api/featured-content/" + missingID(d) + "/view" }}},
			// 本档就是本批修掉的那处塌缩：IncrementViewCount 曾把「查不动」也塌成「内容不存在」。
			{http.StatusInternalServerError, courseFaceCase{method: http.MethodPost, path: func(d courseDomainIDs) string { return "/api/featured-content/" + id(d.featuredPublished) + "/view" }, setup: dropTable("featured_content")}},
		},
	},

	// ===== 学员端 · 课程五个读/写面 =====
	{
		name: "GET /course/:course_id", who: "student",
		cases: []courseDeclaredFace{
			{http.StatusBadRequest, courseFaceCase{method: http.MethodGet, path: func(d courseDomainIDs) string { return "/api/course" + badID }}},
			{http.StatusNotFound, courseFaceCase{method: http.MethodGet, path: func(d courseDomainIDs) string { return "/api/course/" + missingID(d) }}},
			// 本端点**不声明 500 档**：入口第一道是 CourseVisibleByID，它 fail-closed（查不动按
			// 「不可见」，与 QuestionReadScope.VisibleByID 同形先例）⇒ 删表注入在这里必然答 404。
			// 据实不声明，是为了将来把它改成 500 时这一格必须经过这里，而不是悄悄多出一档。
		},
	},
	{
		name: "GET /course/:course_id/chapter/:chapter_id", who: "student",
		cases: []courseDeclaredFace{
			{http.StatusBadRequest, courseFaceCase{method: http.MethodGet, path: func(d courseDomainIDs) string { return "/api/course/" + id(d.course) + "/chapter" + badID }}},
			{http.StatusNotFound, courseFaceCase{method: http.MethodGet, path: func(d courseDomainIDs) string { return "/api/course/" + id(d.course) + "/chapter/" + missingID(d) }}},
			// 章节在、课程行不在 ⇒ 底下事实是「课程不存在」，端点对外仍答「章节不存在」。
			{http.StatusBadRequest, courseFaceCase{method: http.MethodGet, path: func(d courseDomainIDs) string { return "/api/course/" + missingID(d) + "/chapter/" + id(d.chapter) }}},
			{http.StatusInternalServerError, courseFaceCase{method: http.MethodGet, path: func(d courseDomainIDs) string { return "/api/course/" + id(d.course) + "/chapter/" + id(d.chapter) }, setup: dropTable("chapter")}},
		},
	},
	{
		name: "GET /chapter/:chapter_id/slides", who: "student",
		cases: []courseDeclaredFace{
			{http.StatusBadRequest, courseFaceCase{method: http.MethodGet, path: func(d courseDomainIDs) string { return "/api/chapter" + badID + "/slides" }}},
			{http.StatusNotFound, courseFaceCase{method: http.MethodGet, path: func(d courseDomainIDs) string { return "/api/chapter/" + missingID(d) + "/slides" }}},
			{http.StatusInternalServerError, courseFaceCase{method: http.MethodGet, path: func(d courseDomainIDs) string { return "/api/chapter/" + id(d.chapter) + "/slides" }, setup: dropTable("chapter")}},
		},
	},
	{
		name: "POST /chapter/:chapter_id/slides/regenerate", who: "student",
		cases: []courseDeclaredFace{
			{http.StatusBadRequest, courseFaceCase{method: http.MethodPost, path: func(d courseDomainIDs) string { return "/api/chapter" + badID + "/slides/regenerate" }}},
			{http.StatusNotFound, courseFaceCase{method: http.MethodPost, path: func(d courseDomainIDs) string { return "/api/chapter/" + missingID(d) + "/slides/regenerate" }}},
			{http.StatusInternalServerError, courseFaceCase{method: http.MethodPost, path: func(d courseDomainIDs) string { return "/api/chapter/" + id(d.chapter) + "/slides/regenerate" }, setup: dropTable("chapter")}},
		},
	},
	{
		name: "POST /course/:course_id/progress", who: "student",
		cases: []courseDeclaredFace{
			{http.StatusBadRequest, courseFaceCase{method: http.MethodPost, path: func(d courseDomainIDs) string { return "/api/course" + badID + "/progress" }, body: map[string]any{"chapter_id": 1, "duration_seconds": 60}}},
			{http.StatusNotFound, courseFaceCase{method: http.MethodPost, path: func(d courseDomainIDs) string { return "/api/course/" + missingID(d) + "/progress" }, body: map[string]any{"chapter_id": 1, "duration_seconds": 60}}},
			{http.StatusInternalServerError, courseFaceCase{method: http.MethodPost, path: func(d courseDomainIDs) string { return "/api/course/" + id(d.course) + "/progress" }, body: map[string]any{"chapter_id": 1, "duration_seconds": 60}, setup: dropTable("study_record")}},
		},
	},
	{
		name: "GET /student/courses/:course_id", who: "student",
		cases: []courseDeclaredFace{
			{http.StatusBadRequest, courseFaceCase{method: http.MethodGet, path: func(d courseDomainIDs) string { return "/api/student/courses" + badID }}},
			{http.StatusNotFound, courseFaceCase{method: http.MethodGet, path: func(d courseDomainIDs) string { return "/api/student/courses/" + missingID(d) }}},
			{http.StatusInternalServerError, courseFaceCase{method: http.MethodGet, path: func(d courseDomainIDs) string { return "/api/student/courses/" + id(d.course) }, setup: dropTable("course")}},
		},
	},
	{
		// 主体不存在的 token ⇒ 学员档案面按「不存在」答（本端点没有路径 id，故无 400 档）。
		name: "GET /student/profile", who: "ghost",
		cases: []courseDeclaredFace{
			{http.StatusNotFound, courseFaceCase{method: http.MethodGet, path: func(courseDomainIDs) string { return "/api/student/profile" }}},
			{http.StatusInternalServerError, courseFaceCase{method: http.MethodGet, path: func(courseDomainIDs) string { return "/api/student/profile" }, setup: dropTable("hrwai_users")}},
		},
	},

	// ===== 讲师面 =====
	{
		name: "GET /tutor/course/:course_id/chapters", who: "tutor",
		cases: []courseDeclaredFace{
			{http.StatusBadRequest, courseFaceCase{method: http.MethodGet, path: func(d courseDomainIDs) string { return "/api/tutor/course" + badID + "/chapters" }}},
			{http.StatusNotFound, courseFaceCase{method: http.MethodGet, path: func(d courseDomainIDs) string { return "/api/tutor/course/" + missingID(d) + "/chapters" }}},
			{http.StatusInternalServerError, courseFaceCase{method: http.MethodGet, path: func(d courseDomainIDs) string { return "/api/tutor/course/" + id(d.course) + "/chapters" }, setup: dropTable("chapter")}},
		},
	},
	{
		name: "GET /tutor/chapter/:chapter_id", who: "tutor",
		cases: []courseDeclaredFace{
			{http.StatusBadRequest, courseFaceCase{method: http.MethodGet, path: func(d courseDomainIDs) string { return "/api/tutor/chapter" + badID }}},
			{http.StatusNotFound, courseFaceCase{method: http.MethodGet, path: func(d courseDomainIDs) string { return "/api/tutor/chapter/" + missingID(d) }}},
			{http.StatusInternalServerError, courseFaceCase{method: http.MethodGet, path: func(d courseDomainIDs) string { return "/api/tutor/chapter/" + id(d.chapter) }, setup: dropTable("chapter")}},
		},
	},
	{
		name: "PUT /tutor/chapter/:chapter_id", who: "tutor",
		cases: []courseDeclaredFace{
			{http.StatusBadRequest, courseFaceCase{method: http.MethodPut, path: func(d courseDomainIDs) string { return "/api/tutor/chapter" + badID }, body: map[string]any{"title": "台账改章"}}},
			{http.StatusNotFound, courseFaceCase{method: http.MethodPut, path: func(d courseDomainIDs) string { return "/api/tutor/chapter/" + missingID(d) }, body: map[string]any{"title": "台账改章"}}},
			{http.StatusInternalServerError, courseFaceCase{method: http.MethodPut, path: func(d courseDomainIDs) string { return "/api/tutor/chapter/" + id(d.chapter) }, body: map[string]any{"title": "台账改章"}, setup: dropTable("chapter")}},
		},
	},
	{
		name: "DELETE /tutor/file/:file_id", who: "tutor",
		cases: []courseDeclaredFace{
			{http.StatusBadRequest, courseFaceCase{method: http.MethodDelete, path: func(d courseDomainIDs) string { return "/api/tutor/file" + badID }}},
			{http.StatusNotFound, courseFaceCase{method: http.MethodDelete, path: func(d courseDomainIDs) string { return "/api/tutor/file/" + missingID(d) }}},
			{http.StatusInternalServerError, courseFaceCase{method: http.MethodDelete, path: func(d courseDomainIDs) string { return "/api/tutor/file/" + id(d.file) }, setup: dropTable("chapter_file")}},
		},
	},
}

// TestCourseDomainFaceLedger 逐端点逐档打一遍：声明了却打不出即红。
//
// 这一条同时是「档位集合与代码一致」的锁：删掉某端点的一条 WithSentinel，它对应的 404/400
// 档会当场打不出来而红（本批实测过，见 ADR-0064 实施回记）。
func TestCourseDomainFaceLedger(t *testing.T) {
	for _, ep := range courseDomainFaces {
		for i, declared := range ep.cases {
			c := declared.req
			t.Run(fmt.Sprintf("%s/%d#%d", ep.name, declared.want, i), func(t *testing.T) {
				r, db, adminToken := newAdminContractEnv(t)
				ids := seedCourseDomain(t, db)
				token := adminToken
				if ep.who != "admin" {
					token = tokenFor(t, db, ep.who)
				}
				if c.setup != nil {
					c.setup(t, db)
				}
				rec := doWithToken(t, r, token, c.method, c.path(ids), c.body)
				if rec.Code != declared.want {
					t.Fatalf("%s：声明档位 %d 打不出来，实得 %d，body=%s", ep.name, declared.want, rec.Code, rec.Body.String())
				}
				if body := rec.Body.String(); leakyDriverText(body) {
					t.Fatalf("%s 把驱动原文吐进响应体（ADR-0064 决策 2 的泄漏族）: %s", ep.name, body)
				}
			})
		}
	}
}
