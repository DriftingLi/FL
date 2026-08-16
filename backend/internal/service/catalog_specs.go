// Package service 课程目录实体的 descriptor 定义（ADR-0015）。
// 每个 descriptor 只声明该实体的字段差异；共享行为在 catalog engine 中实现。
package service

import (
	"errors"

	"forklift-training/internal/model"
)

func specialtyCatalogSpec() CatalogEntitySpec[model.Specialty, SpecialtyInput, SpecialtyDict] {
	return CatalogEntitySpec[model.Specialty, SpecialtyInput, SpecialtyDict]{
		Table:       "specialty",
		IDColumn:    "specialty_id",
		OrderBy:     "sort_order ASC, specialty_id ASC",
		CodeErr:     "专业方向编码不能为空",
		NameErr:     "专业方向名称不能为空",
		DupMsg:      "专业方向编码已存在",
		NotFoundMsg: "专业方向不存在",
		Sortable:    true,
		Code:        func(in *SpecialtyInput) string { return in.Code },
		ModelCode:   func(m *model.Specialty) string { return m.Code },
		Name:        func(in *SpecialtyInput) string { return in.Name },
		SortOrder:   func(in *SpecialtyInput) *int { return in.SortOrder },
		Status:      func(in *SpecialtyInput) *int16 { return in.Status },
		EmptyModel:  func() any { return &model.Specialty{} },
		NewModel: func(in *SpecialtyInput, sortOrder int) model.Specialty {
			return model.Specialty{
				Code:        in.Code,
				Name:        in.Name,
				Description: inputString(in.Description),
				SortOrder:   sortOrder,
				Status:      inputInt16(in.Status, 1),
				CreatedAt:   beijingNow(),
			}
		},
		ApplyUpdate: func(m *model.Specialty, in *SpecialtyInput) {
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
		SetCode: func(m *model.Specialty, code string) { m.Code = code },
		SetName: func(m *model.Specialty, name string) { m.Name = name },
		ToDict:  specialtyDict,
	}
}

func levelCatalogSpec() CatalogEntitySpec[model.CourseLevel, LevelInput, LevelDict] {
	return CatalogEntitySpec[model.CourseLevel, LevelInput, LevelDict]{
		Table:       "course_level",
		IDColumn:    "level_id",
		OrderBy:     "sort_order ASC, level_id ASC",
		CodeErr:     "课程等级编码不能为空",
		NameErr:     "课程等级名称不能为空",
		DupMsg:      "课程等级编码已存在",
		NotFoundMsg: "课程等级不存在",
		Sortable:    true,
		Code:        func(in *LevelInput) string { return in.Code },
		ModelCode:   func(m *model.CourseLevel) string { return m.Code },
		Name:        func(in *LevelInput) string { return in.Name },
		SortOrder:   func(in *LevelInput) *int { return in.SortOrder },
		Status:      func(in *LevelInput) *int16 { return in.Status },
		EmptyModel:  func() any { return &model.CourseLevel{} },
		NewModel: func(in *LevelInput, sortOrder int) model.CourseLevel {
			return model.CourseLevel{
				Code:        in.Code,
				Name:        in.Name,
				Description: inputString(in.Description),
				SortOrder:   sortOrder,
				Status:      inputInt16(in.Status, 1),
				CreatedAt:   beijingNow(),
			}
		},
		ApplyUpdate: func(m *model.CourseLevel, in *LevelInput) {
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
		SetCode: func(m *model.CourseLevel, code string) { m.Code = code },
		SetName: func(m *model.CourseLevel, name string) { m.Name = name },
		ToDict:  levelDict,
	}
}

func certificateCatalogSpec() CatalogEntitySpec[model.CertificateTemplate, CertificateTemplateInput, CertificateTemplateDict] {
	return CatalogEntitySpec[model.CertificateTemplate, CertificateTemplateInput, CertificateTemplateDict]{
		Table:       "certificate_template",
		IDColumn:    "id",
		OrderBy:     "id ASC",
		CodeErr:     "证书模板编码不能为空",
		NameErr:     "证书模板名称不能为空",
		DupMsg:      "证书模板编码已存在",
		NotFoundMsg: "证书模板不存在",
		Sortable:    false,
		Code:        func(in *CertificateTemplateInput) string { return in.Code },
		ModelCode:   func(m *model.CertificateTemplate) string { return m.Code },
		Name:        func(in *CertificateTemplateInput) string { return in.Name },
		Status:      func(in *CertificateTemplateInput) *int16 { return in.Status },
		Validate: func(in *CertificateTemplateInput) error {
			if in.ValidityDays != nil && *in.ValidityDays <= 0 {
				return errors.New("证书有效期必须为正整数（天）")
			}
			return nil
		},
		EmptyModel: func() any { return &model.CertificateTemplate{} },
		NewModel: func(in *CertificateTemplateInput, _ int) model.CertificateTemplate {
			now := beijingNow()
			return model.CertificateTemplate{
				Code:         in.Code,
				Name:         in.Name,
				Description:  inputString(in.Description),
				ValidityDays: inputInt(in.ValidityDays, 365),
				TemplateURL:  inputString(in.TemplateURL),
				Status:       inputInt16(in.Status, 1),
				CreatedAt:    now,
				UpdatedAt:    now,
			}
		},
		ApplyUpdate: func(m *model.CertificateTemplate, in *CertificateTemplateInput) {
			if in.Description != nil {
				m.Description = *in.Description
			}
			if in.ValidityDays != nil {
				m.ValidityDays = *in.ValidityDays
			}
			if in.TemplateURL != nil {
				m.TemplateURL = *in.TemplateURL
			}
			if in.Status != nil {
				m.Status = *in.Status
			}
			m.UpdatedAt = beijingNow()
		},
		SetCode: func(m *model.CertificateTemplate, code string) { m.Code = code },
		SetName: func(m *model.CertificateTemplate, name string) { m.Name = name },
		ToDict:  certTemplateDict,
	}
}

func questionTagCatalogSpec() CatalogEntitySpec[model.QuestionTag, QuestionTagInput, QuestionTagDict] {
	return CatalogEntitySpec[model.QuestionTag, QuestionTagInput, QuestionTagDict]{
		Table:       "question_tag",
		IDColumn:    "id",
		OrderBy:     "sort_order ASC, id ASC",
		CodeErr:     "标签编码不能为空",
		NameErr:     "标签名称不能为空",
		DupMsg:      "标签编码已存在",
		NotFoundMsg: "题库标签不存在",
		Sortable:    true,
		Code:        func(in *QuestionTagInput) string { return in.Code },
		ModelCode:   func(m *model.QuestionTag) string { return m.Code },
		Name:        func(in *QuestionTagInput) string { return in.Name },
		SortOrder:   func(in *QuestionTagInput) *int { return in.SortOrder },
		Status:      func(in *QuestionTagInput) *int16 { return in.Status },
		EmptyModel:  func() any { return &model.QuestionTag{} },
		NewModel: func(in *QuestionTagInput, sortOrder int) model.QuestionTag {
			now := beijingNow()
			return model.QuestionTag{
				Code:        in.Code,
				Name:        in.Name,
				Description: inputString(in.Description),
				SortOrder:   sortOrder,
				Status:      inputInt16(in.Status, 1),
				CreatedAt:   now,
				UpdatedAt:   now,
			}
		},
		ApplyUpdate: func(m *model.QuestionTag, in *QuestionTagInput) {
			if in.Description != nil {
				m.Description = *in.Description
			}
			if in.SortOrder != nil {
				m.SortOrder = *in.SortOrder
			}
			if in.Status != nil {
				m.Status = *in.Status
			}
			m.UpdatedAt = beijingNow()
		},
		SetCode: func(m *model.QuestionTag, code string) { m.Code = code },
		SetName: func(m *model.QuestionTag, name string) { m.Name = name },
		ToDict:  tagDict,
	}
}
