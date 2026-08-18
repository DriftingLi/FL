// Package service 课程目录 CRUD engine：四个目录实体共享的
// List/Create/Update/Delete/Swap 实现。descriptor 使用 typed 回调，
// 不引入 generic map（ADR-0015）。engine 是 internal seam，不对外暴露。
package service

import (
	"errors"

	"gorm.io/gorm"
)

// CatalogEntitySpec 描述一个课程目录实体的持久化与字段差异。
type CatalogEntitySpec[M any, I any, D any] struct {
	Table       string
	IDColumn    string
	OrderBy     string
	CodeErr     string
	NameErr     string
	DupMsg      string
	NotFoundMsg string

	// Sortable 为 false 时实体没有 sort_order（证书模板）。
	Sortable bool

	Code      func(*I) string
	ModelCode func(*M) string
	Name      func(*I) string
	SortOrder func(*I) *int
	Status    func(*I) *int16

	// Validate 承接 code/name/status 之外的字段校验（如证书有效期）。
	Validate func(*I) error
	// EmptyModel 返回空 model 指针（排序/删除需要具体表类型）。
	EmptyModel func() any
	// NewModel 创建 model；sortOrder 对 Sortable=true 的实体有效。
	NewModel func(*I, int) M
	// ApplyUpdate 应用 code/name 之外的字段更新。
	ApplyUpdate func(*M, *I)
	// SetCode / SetName 写入通用字段。
	SetCode func(*M, string)
	SetName func(*M, string)
	// ToDict 是实体 typed facade 的返回形状。
	ToDict func(*M) D
}

func catalogList[M any, I any, D any](db *gorm.DB, spec CatalogEntitySpec[M, I, D], activeOnly bool) []D {
	q := db.Model(spec.EmptyModel())
	if activeOnly {
		q = q.Where("status = ?", 1)
	}
	var rows []M
	q.Order(spec.OrderBy).Find(&rows)
	items := make([]D, 0, len(rows))
	for i := range rows {
		items = append(items, spec.ToDict(&rows[i]))
	}
	return items
}

func catalogCreate[M any, I any, D any](db *gorm.DB, spec CatalogEntitySpec[M, I, D], in *I) (D, error) {
	var zero D
	if err := validateCatalogCodeName(spec.Code(in), spec.Name(in), spec.CodeErr, spec.NameErr); err != nil {
		return zero, err
	}
	if err := validateStatus(spec.Status(in)); err != nil {
		return zero, err
	}
	if spec.Validate != nil {
		if err := spec.Validate(in); err != nil {
			return zero, err
		}
	}
	if err := ensureCodeUnique(db, spec.Table, spec.IDColumn, spec.Code(in), 0, spec.DupMsg); err != nil {
		return zero, err
	}

	sortOrder := 0
	if spec.Sortable {
		sortOrder = inputInt(spec.SortOrder(in), nextSortOrderValue(db, spec.Table, nil))
	}
	m := spec.NewModel(in, sortOrder)
	if err := db.Create(&m).Error; err != nil {
		return zero, err
	}
	return spec.ToDict(&m), nil
}

func catalogUpdate[M any, I any, D any](db *gorm.DB, spec CatalogEntitySpec[M, I, D], id int, in *I) (D, error) {
	var zero D
	var m M
	if err := db.First(&m, id).Error; err != nil {
		return zero, errors.New(spec.NotFoundMsg)
	}
	if err := validateStatus(spec.Status(in)); err != nil {
		return zero, err
	}
	if spec.Validate != nil {
		if err := spec.Validate(in); err != nil {
			return zero, err
		}
	}

	code := spec.Code(in)
	if code != "" {
		if spec.ModelCode(&m) != "" && code != spec.ModelCode(&m) {
			if err := ensureCodeUnique(db, spec.Table, spec.IDColumn, code, id, spec.DupMsg); err != nil {
				return zero, err
			}
		}
		spec.SetCode(&m, code)
	}
	if name := spec.Name(in); name != "" {
		spec.SetName(&m, name)
	}
	if spec.ApplyUpdate != nil {
		spec.ApplyUpdate(&m, in)
	}
	if err := db.Save(&m).Error; err != nil {
		return zero, err
	}
	return spec.ToDict(&m), nil
}

func catalogDelete[M any, I any, D any](db *gorm.DB, spec CatalogEntitySpec[M, I, D], id int) error {
	var m M
	result := db.Delete(&m, id)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return errors.New(spec.NotFoundMsg)
	}
	return nil
}

func catalogSwap[M any, I any, D any](db *gorm.DB, spec CatalogEntitySpec[M, I, D], a, b int) error {
	if !spec.Sortable {
		return errors.New("该实体不支持排序交换")
	}
	return swapGroupPositions(db, spec.EmptyModel(), spec.IDColumn, a, b, nil)
}
