// 「不可读」的四件事实必须**互相不可替换**（ADR-0064 决策 1；词表「不可读及其成因」）。
//
// 这条测试是为本波自己犯的一个错而写的：拆分初稿把哨兵写成了
//
//	ErrCourseNotVisible = ErrChapterNotFound
//	ErrCourseLocked     = ErrChapterNotFound
//
// —— 编译通过、四条 404 映射测试全绿，但 errors.Is 把四态认成同一件，分档在运行期**完全没发生**。
// 也就是说：只验「对外码一致」的测试对这项改动是无效验证。下面第一组断言钉的就是这个洞。
//
// 呈现层仍然统一：四件事实由各消费端点显式映射成 404 + 该端点自己的那句话
// （ADR-0062 决策 3 的不泄漏口径）。本文件验的是**类型层分开**，不是码不同。
package service

import (
	"errors"
	"strconv"
	"testing"

	"go.uber.org/zap"

	"forklift-training/internal/model"
	"forklift-training/internal/testutil"
)

// TestCourseReadabilityFactsAreDistinct 两两不可替换（error value 与文案各一层）。
func TestCourseReadabilityFactsAreDistinct(t *testing.T) {
	facts := map[string]error{
		"真不存在(课程)": ErrCourseNotFound,
		"真不存在(章节)": ErrChapterNotFound,
		"不在平台上":    ErrCourseNotVisible,
		"无权益":      ErrCourseLocked,
	}
	names := []string{"真不存在(课程)", "真不存在(章节)", "不在平台上", "无权益"}
	for i := range names {
		for j := range names {
			if i == j {
				continue
			}
			if errors.Is(facts[names[i]], facts[names[j]]) {
				t.Fatalf("「%s」与「%s」在类型层可互相顶替 ⇒ 拆分失效（别名？）", names[i], names[j])
			}
		}
	}
	// 文案那一层同判：四件事实必须各自说得出自己是谁。
	// 拆分初稿把三件的 Error() 都写成「章节不存在」（为了让对外只有一句话），后果是
	// 「未兑换」与「不可见」在日志里同名单、课程面与章节面说的对象名互串 —— 那是把呈现
	// 决定写进判据层。对外统一的那句话改由端点给（api 层 WithSentinelsMsg）。
	seen := map[string]string{}
	for _, n := range names {
		msg := facts[n].Error()
		if prev, dup := seen[msg]; dup {
			t.Fatalf("「%s」与「%s」文案同为「%s」⇒ 事实分档在可读面上又塌回一件", n, prev, msg)
		}
		seen[msg] = n
	}
}

// TestStudentCanReadCoursePicksTheRightFact 每种状态真的抛出**那一个**，不是笼统「不可读」。
// 夹具照 course_chapter_detail_test.go 的既有形状（课程须「已发布 + 已挂载」才走得到权益那一半）。
func TestStudentCanReadCoursePicksTheRightFact(t *testing.T) {
	db := testutil.NewMemoryDB(t)
	svc := NewCourseService(db, nil, zap.NewNop())
	student := testutil.SeedStudent(t, db, "readscope_stu", "hash")

	spec := model.Specialty{Code: "readscope", Name: "读档", SortOrder: 1, Status: 1}
	if err := db.Create(&spec).Error; err != nil {
		t.Fatalf("创建方向失败: %v", err)
	}
	lv := model.CourseLevel{Code: "readscope-lv", Name: "入门", SortOrder: 1, Status: 1}
	if err := db.Create(&lv).Error; err != nil {
		t.Fatalf("创建等级失败: %v", err)
	}

	// 1) 课程行根本不在。
	if err := svc.studentCanReadCourse(999999, student.ID); !errors.Is(err, ErrCourseNotFound) {
		t.Fatalf("课程不存在应报 ErrCourseNotFound，实际 %v", err)
	}

	// 2) 行在、但未挂载（不满足 ADR-0058 的挂载不变式）= 不在平台上。
	unmounted := model.Course{Name: "未挂载课", Status: 1, CreatedAt: testutil.Now()}
	if err := db.Create(&unmounted).Error; err != nil {
		t.Fatalf("播种未挂载课程失败: %v", err)
	}
	if err := svc.studentCanReadCourse(unmounted.CourseID, student.ID); !errors.Is(err, ErrCourseNotVisible) {
		t.Fatalf("未挂载应报 ErrCourseNotVisible，实际 %v", err)
	}

	// 3) 已挂载 + 已发布 + 定价，但这个人没兑换 = 无权益（第三件独立事实）。
	price := 300
	priced := model.Course{Name: "付费课", Status: 1, PointsPrice: &price,
		SpecialtyID: &spec.SpecialtyID, LevelID: &lv.LevelID, CreatedAt: testutil.Now()}
	if err := db.Create(&priced).Error; err != nil {
		t.Fatalf("播种付费课程失败: %v", err)
	}
	if err := svc.studentCanReadCourse(priced.CourseID, student.ID); !errors.Is(err, ErrCourseLocked) {
		t.Fatalf("未兑换应报 ErrCourseLocked（与「不存在」是两件事），实际 %v", err)
	}

	// 4) 同一门课，兑换之后判据放行 —— 证明第 3 条不是恒假的死闸。
	ent := model.UserEntitlement{UserID: student.ID, SKU: CourseSKU(priced.CourseID),
		RefID: strconv.Itoa(priced.CourseID)}
	if err := db.Create(&ent).Error; err != nil {
		t.Fatalf("播种权益行失败: %v", err)
	}
	if err := svc.studentCanReadCourse(priced.CourseID, student.ID); err != nil {
		t.Fatalf("已兑换仍不可读 ⇒ 权益被做成了死闸: %v", err)
	}
}
