// Package service 实现业务服务层。
// 本文件：课程挂载不变式（ADR-0006 口径，ADR-0050 决策 1 单点化）——唯一事实源。
//
// 领域规则：课程必须同时挂专业方向与课程等级才「存在/可见」——
// 学员端列表与目录树只展示已挂载课程；创建/编辑/排序校验同源。
//
// interface 三形态同源：
//   - typed 判定 CourseMounted：写入校验、目录树过滤等内存判定；
//   - SQL 形态 MountedCourseScope：查询侧可见性谓词（学员列表、搜索课程/章节分区、收藏目标校验）；
//   - by-id 判定 CourseVisibleByID：**按 id 读路径**的可见性判定（课程详情 / 章节详情 / 章节幻灯片，
//     ADR-0058 —— 对齐 ADR-0049 对题目域已确立的「可见性覆盖每一条读路径」）。
//
// 「已发布（status = 1）」不是挂载不变式的一部分：调用方按读面语义另行叠加
// （CourseVisibleByID 是本单点里唯一把两者叠好的形态，读面一律用它，不手拼谓词）。
package service

import (
	"gorm.io/gorm"

	"forklift-training/internal/model"
)

// CourseMounted 挂载判定（typed 形态）。
func CourseMounted(specialtyID, levelID *int) bool {
	return specialtyID != nil && levelID != nil
}

// MountedCourseScope 学员端可见课程查询范围：挂载不变式的 SQL 形态。
func MountedCourseScope(q *gorm.DB) *gorm.DB {
	return q.Where("specialty_id IS NOT NULL AND level_id IS NOT NULL")
}

// CourseVisibleByID 学员可见性判定的 by-id 形态：课程必须**已发布（status = 1）且已挂载**。
// 供按 id 取内容的读路径复用（ADR-0058）；不可见一律按「不存在」返回，调用方渲染 404 空态。
func CourseVisibleByID(db *gorm.DB, courseID int) bool {
	var cnt int64
	MountedCourseScope(db.Model(&model.Course{}).Where("course_id = ? AND status = 1", courseID)).Count(&cnt)
	return cnt > 0
}
