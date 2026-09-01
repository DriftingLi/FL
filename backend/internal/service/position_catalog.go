// Package service 岗位字典（问题4：岗位与专业方向解绑，管理员配置）。
// 复用课程目录 catalog 引擎（ADR-0015 descriptor 驱动），与专业方向/等级同构。
package service

import (
	"forklift-training/internal/model"
)

// PositionInput 岗位创建/更新入参。
// 更新语义（与既有 catalog 一致）：Code/Name 为空表示不改动；指针字段为 nil 表示不改动。
type PositionInput struct {
	Code        string  `json:"code"`
	Name        string  `json:"name"`
	Description *string `json:"description"`
	SortOrder   *int    `json:"sort_order"`
	Status      *int16  `json:"status"`
}

// PositionDict 岗位字典项。
type PositionDict struct {
	Code        string `json:"code"`
	CreatedAt   string `json:"created_at"`
	Description string `json:"description"`
	Name        string `json:"name"`
	PositionID  int    `json:"position_id"`
	SortOrder   int    `json:"sort_order"`
	Status      int16  `json:"status"`
}

// positionCatalogSpec 岗位字典的 catalog descriptor。
func positionCatalogSpec() CatalogEntitySpec[model.Position, PositionInput, PositionDict] {
	return CatalogEntitySpec[model.Position, PositionInput, PositionDict]{
		Table:       "positions",
		IDColumn:    "position_id",
		OrderBy:     "sort_order ASC, position_id ASC",
		CodeErr:     "岗位编码不能为空",
		NameErr:     "岗位名称不能为空",
		DupMsg:      "岗位编码已存在",
		NotFoundMsg: "岗位不存在",
		Sortable:    true,
		Code:        func(in *PositionInput) string { return in.Code },
		ModelCode:   func(m *model.Position) string { return m.Code },
		Name:        func(in *PositionInput) string { return in.Name },
		SortOrder:   func(in *PositionInput) *int { return in.SortOrder },
		Status:      func(in *PositionInput) *int16 { return in.Status },
		EmptyModel:  func() any { return &model.Position{} },
		NewModel: func(in *PositionInput, sortOrder int) model.Position {
			return model.Position{
				Code:        in.Code,
				Name:        in.Name,
				Description: inputString(in.Description),
				SortOrder:   sortOrder,
				Status:      inputInt16(in.Status, 1),
				CreatedAt:   beijingNow(),
			}
		},
		ApplyUpdate: func(m *model.Position, in *PositionInput) {
			if in.Description != nil {
				m.Description = *in.Description
			}
			if in.SortOrder != nil {
				m.SortOrder = *in.SortOrder
			}
			if in.Status != nil {
				m.Status = *in.Status
			}
		},
		SetCode: func(m *model.Position, code string) { m.Code = code },
		SetName: func(m *model.Position, name string) { m.Name = name },
		ToDict: func(m *model.Position) PositionDict {
			return PositionDict{
				Code:        m.Code,
				CreatedAt:   formatISO(m.CreatedAt),
				Description: m.Description,
				Name:        m.Name,
				PositionID:  m.PositionID,
				SortOrder:   m.SortOrder,
				Status:      m.Status,
			}
		},
	}
}

// ListPositions 岗位列表（管理端含停用项，学员/招聘端仅启用项）。
func (s *TrainingCatalogService) ListPositions(activeOnly bool) []PositionDict {
	return catalogList(s.db, positionCatalogSpec(), activeOnly)
}

// CreatePosition 创建岗位。
func (s *TrainingCatalogService) CreatePosition(in PositionInput) (PositionDict, error) {
	return catalogCreate(s.db, positionCatalogSpec(), &in)
}

// SwapPositionSort 交换两个岗位的排序位置。
func (s *TrainingCatalogService) SwapPositionSort(a, b int) error {
	return catalogSwap(s.db, positionCatalogSpec(), a, b)
}

// UpdatePosition 更新岗位。
func (s *TrainingCatalogService) UpdatePosition(id int, in PositionInput) (PositionDict, error) {
	return catalogUpdate(s.db, positionCatalogSpec(), id, &in)
}

// DeletePosition 删除岗位（已关联职位/简历置空 position_id，不级联删除）。
func (s *TrainingCatalogService) DeletePosition(id int) error {
	return catalogDelete(s.db, positionCatalogSpec(), id)
}
