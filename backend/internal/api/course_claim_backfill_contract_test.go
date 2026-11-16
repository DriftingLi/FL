// 契约测试 #2596 B7：课程认领存量回填（pg 双跑）。
//
// 迁移 000059 内以「章节作者归属非空的课程」为其归属讲师回填认领行（来源记 self），
// 本测试把样本打回「迁移前形状」（只有 chapter.tutor_id，无认领行）后重放同一句 SQL，
// 验三件事：
//
//	有作者的课程 → 生成认领行；无主课程 → 不回填（保持开放认领）；重放幂等不重复。
package api

import (
	"testing"

	"forklift-training/internal/model"
	"forklift-training/internal/testutil"
)

// backfillCourseClaimsSQL 与 migrations/000059_tutor_course_claim.up.sql 的最后一条语句逐字一致。
// 抄一份进测试是有意的：回填判据（哪些课该回填）是彩票级事实，重放它才能验幂等与「无主不回填」。
// 漂移由 migrations 侧的存在性保证——迁移文件删了这句，回填就永远不发生，本测试仍绿（反向锁在
// 迁移登记表的那一层，不在本文件）。
const backfillCourseClaimsSQL = `
INSERT INTO tutor_course_claim (tutor_id, course_id, source)
SELECT DISTINCT tutor_id, course_id, 'self'
FROM chapter
WHERE tutor_id IS NOT NULL
ON CONFLICT (tutor_id, course_id) DO NOTHING`

func TestCourseClaimBackfillOnPostgres(t *testing.T) {
	testutil.SetTestGinMode()
	db := testutil.NewPostgresDB(t)
	if db == nil {
		t.Skip("DATABASE_URL 未设置")
	}

	ptr := func(v int) *int { return &v }
	spec := model.Specialty{Code: "claim-backfill", Name: "认领回填", SortOrder: 1, Status: 1}
	if err := db.Create(&spec).Error; err != nil {
		t.Fatalf("建方向失败: %v", err)
	}
	lv := model.CourseLevel{Code: "claim-backfill-lv", Name: "入门", SortOrder: 1, Status: 1}
	if err := db.Create(&lv).Error; err != nil {
		t.Fatalf("建等级失败: %v", err)
	}
	mkCourse := func(name string) *model.Course {
		c := model.Course{Name: name, Status: 1,
			SpecialtyID: ptr(spec.SpecialtyID), LevelID: ptr(lv.LevelID), CreatedAt: testutil.Now()}
		if err := db.Create(&c).Error; err != nil {
			t.Fatalf("建课程 %s 失败: %v", name, err)
		}
		return &c
	}
	mkChapter := func(courseID int, tutorID *int, title string) {
		ch := model.Chapter{CourseID: courseID, Title: title, ContentType: "text",
			OrderNum: 1, TutorID: tutorID, CreatedAt: testutil.Now()}
		if err := db.Create(&ch).Error; err != nil {
			t.Fatalf("建章节 %s 失败: %v", title, err)
		}
	}

	// 迁移前形状：一门课有多章同属一位讲师（回填要去重成一行），
	// 另一门课有一章归属讲师 2、一章无主（该课仍应回填讲师 2），
	// 第三门课全部无主（不回填，保持开放认领）。
	owned := mkCourse("有主课程")
	mkChapter(owned.CourseID, ptr(1), "章一")
	mkChapter(owned.CourseID, ptr(1), "章二") // 同一讲师两章 → DISTINCT 后仍一行

	mixed := mkCourse("混合课程")
	mkChapter(mixed.CourseID, ptr(2), "有主章")
	mkChapter(mixed.CourseID, nil, "无主章")

	ownerless := mkCourse("无主课程")
	mkChapter(ownerless.CourseID, nil, "无主章一")
	mkChapter(ownerless.CourseID, nil, "无主章二")

	// 迁移语义：本测试前认领表可能是空的（新 schema）也可能有行，先清空到「迁移前形状」。
	if err := db.Where("course_id IN ?", []int{owned.CourseID, mixed.CourseID, ownerless.CourseID}).
		Delete(&model.TutorCourseClaim{}).Error; err != nil {
		t.Fatalf("清空认领行失败: %v", err)
	}

	// 重放迁移里的回填语句
	replay := func() {
		t.Helper()
		if err := db.Exec(backfillCourseClaimsSQL).Error; err != nil {
			t.Fatalf("重放回填失败: %v", err)
		}
	}
	countClaims := func(courseID int) int64 {
		t.Helper()
		var n int64
		if err := db.Model(&model.TutorCourseClaim{}).Where("course_id = ?", courseID).Count(&n).Error; err != nil {
			t.Fatalf("查认领行失败: %v", err)
		}
		return n
	}

	replay()

	// 有主课程：两章同主 → 一行，且来源记 self
	var got model.TutorCourseClaim
	if err := db.Where("course_id = ?", owned.CourseID).First(&got).Error; err != nil {
		t.Fatalf("有主课程未回填认领行: %v", err)
	}
	if got.TutorID != 1 || got.Source != model.ClaimSourceSelf {
		t.Fatalf("回填行 = %+v, want tutor_id=1 source=self", got)
	}
	if n := countClaims(owned.CourseID); n != 1 {
		t.Fatalf("同讲师多章应只回填一行, got %d", n)
	}

	// 混合课程：只回填讲师 2 那一行
	if n := countClaims(mixed.CourseID); n != 1 {
		t.Fatalf("混合课程应回填 1 行（只算有主章）, got %d", n)
	}

	// 无主课程：不回填（保持开放认领）
	if n := countClaims(ownerless.CourseID); n != 0 {
		t.Fatalf("无主课程不应回填, got %d 行", n)
	}

	// 幂等：重放不重复
	replay()
	if n := countClaims(owned.CourseID); n != 1 {
		t.Fatalf("重放后同讲师同课程应仍 1 行（幂等）, got %d", n)
	}
	if n := countClaims(mixed.CourseID); n != 1 {
		t.Fatalf("重放后混合课程应仍 1 行, got %d", n)
	}

	// 唯一键兜底：绕过回填直接插重复行应被拒（ON CONFLICT 之外的第二道保险）
	dup := model.TutorCourseClaim{TutorID: 1, CourseID: owned.CourseID, Source: model.ClaimSourceAdmin, CreatedAt: testutil.Now()}
	if err := db.Create(&dup).Error; err == nil {
		t.Fatal("重复 (tutor_id, course_id) 应被唯一索引拒绝")
	}
}
