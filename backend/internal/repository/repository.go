// Package repository 提供数据访问层。
//
// 仓储层封装每个聚合根的 CRUD 与业务查询，供 service 层调用。
package repository

import (
	"gorm.io/gorm"

	"forklift-training/internal/model"
)

// StudentRepository 学员仓储。
type StudentRepository struct {
	db *gorm.DB
}

// NewStudentRepository 创建学员仓储。
func NewStudentRepository(db *gorm.DB) *StudentRepository {
	return &StudentRepository{db: db}
}

// FindByID 按 ID 查询学员。
func (r *StudentRepository) FindByID(id int) (*model.Student, error) {
	var s model.Student
	if err := r.db.First(&s, id).Error; err != nil {
		return nil, err
	}
	return &s, nil
}

// FindByUsername 按用户名查询学员。
func (r *StudentRepository) FindByUsername(username string) (*model.Student, error) {
	var s model.Student
	if err := r.db.Where("username = ?", username).First(&s).Error; err != nil {
		return nil, err
	}
	return &s, nil
}

// Create 创建学员。
func (r *StudentRepository) Create(s *model.Student) error {
	return r.db.Create(s).Error
}

// Update 更新学员。
func (r *StudentRepository) Update(s *model.Student) error {
	return r.db.Save(s).Error
}

// Paginate 通用分页结果。
type Paginate[T any] struct {
	Items []T   `json:"items"`
	Total int64 `json:"total"`
	Page  int   `json:"page"`
	Size  int   `json:"size"`
}
