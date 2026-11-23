package course

import (
	"gorm.io/gorm"

	"forklift-training/internal/model"
)

// CourseMounted 判定课程能否归入旧目录的方向与等级节点。
func CourseMounted(specialtyID, levelID *int) bool {
	return specialtyID != nil && levelID != nil
}

func PublishedCategoryCourseScope(q *gorm.DB) *gorm.DB {
	return q.Where("course.status = ?", 1).
		Where("EXISTS (SELECT 1 FROM training_category WHERE training_category.id = course.category_id)")
}

// CourseVisibleByID 学员可见性判定的 by-id 形态：课程必须已发布且存在分类。
// 供按 id 取内容的读路径复用（ADR-0058）；真不可见按「不存在」返回，调用方渲染 404 空态。
//
// (false, nil) 与 (false, err) 是两件事实（ADR-0065 决策 7）：前者是「平台证明它不可见」，
// 后者是「平台问不出可见性」。旧签名把 err 整个丢弃、一律 fail-closed 成 false，于是读不动
// courses 表时对外答 404「课程不存在」——调用方会把它当空态缓存下来，故障被演成「这门课没了」。
// fail-closed 的**保守方向**仍然成立：出错时返回 false，调用方不得把内容交出去。
func CourseVisibleByID(db *gorm.DB, courseID int) (bool, error) {
	var cnt int64
	if err := PublishedCategoryCourseScope(db.Model(&model.Course{}).Where("course_id = ?", courseID)).Count(&cnt).Error; err != nil {
		return false, err
	}
	return cnt > 0, nil
}
