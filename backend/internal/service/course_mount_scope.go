// Package service 实现业务服务层。
// 本文件：课程挂载不变式（ADR-0006 口径，ADR-0050 决策 1 单点化）——唯一事实源。
//
// 领域规则：课程必须同时挂专业方向与课程等级才「存在/可见」——
// 学员端列表与目录树只展示已挂载课程；创建/编辑/排序校验同源。
//
// interface 双形态同源：
//   - typed 判定 CourseMounted：写入校验、目录树过滤等内存判定；
//   - SQL 形态 MountedCourseScope：查询侧可见性谓词（学员列表、搜索课程/章节分区、收藏目标校验）。
//
// 「已发布（status = 1）」不是挂载不变式的一部分：调用方按读面语义另行叠加。
package service

import "gorm.io/gorm"

// CourseMounted 挂载判定（typed 形态）。
func CourseMounted(specialtyID, levelID *int) bool {
	return specialtyID != nil && levelID != nil
}

// MountedCourseScope 学员端可见课程查询范围：挂载不变式的 SQL 形态。
func MountedCourseScope(q *gorm.DB) *gorm.DB {
	return q.Where("specialty_id IS NOT NULL AND level_id IS NOT NULL")
}
