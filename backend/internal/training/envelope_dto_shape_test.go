// spec #954 片二：handler **内联构造**的响应 map 定型为 DTO 后的同一套字节锁。
//
// 本文件原在 internal/service（#1445 P2 波 3b-2 随培训域搬来）：表内 13 例全是培训目录 /
// 目标证件 / 岗位面的 envelope DTO —— 证据跟真实出口走，不跟类型名前缀走（ADR-0070 决策 9）。
// 左边是**改造前的 map 形态**（不是手抄的 JSON 字面量，否则抄错即与 DTO 同错），右边是收口后的
// DTO：encoding/json 对 map 按 key 排序、对 struct 按字段声明序输出，所以字段顺序写错、加了
// omitempty、或把空切片写成 nil，这里立刻红。题库 / 练习 / 招聘 / 诊断面的用例仍留在原文件。
package training

import (
	"encoding/json"
	"testing"
)

// intPtr 本地 helper（先例：每个测试文件自带同名 helper，不去共享）。
func intPtr(v int) *int { return &v }

// int64Ptr 构造 *int64（QuestionTagDict.question_count 的「键在 / 键缺」两态用例）。
func int64Ptr(v int64) *int64 { return &v }

func TestInlineResponseDTOBytes(t *testing.T) {
	cases := []struct {
		name   string
		legacy any
		dto    any
	}{
		{
			name:   "QuestionTagsResultDTO",
			legacy: map[string]any{"tag_ids": []int{3, 5}},
			dto:    &QuestionTagsResultDTO{TagIDs: []int{3, 5}},
		},
		{
			name:   "QuestionTagsResultDTO（无标签：nil 切片仍是 null）",
			legacy: map[string]any{"tag_ids": nil},
			dto:    &QuestionTagsResultDTO{},
		},
		{
			name: "LevelListDTO（GET /levels 与 /admin/levels：原 gin.H 的 levels 键）",
			legacy: map[string]any{"levels": []LevelDict{{
				Code: "L1", CreatedAt: "2026-09-13T10:00:00.000000+08:00", Description: "初级",
				LevelID: 1, Name: "初级", SortOrder: 1, Status: 1,
			}}},
			dto: &LevelListDTO{Levels: []LevelDict{{
				Code: "L1", CreatedAt: "2026-09-13T10:00:00.000000+08:00", Description: "初级",
				LevelID: 1, Name: "初级", SortOrder: 1, Status: 1,
			}}},
		},
		{
			name: "QuestionTagListDTO（GET /tags 与 /admin/question-tags：原 gin.H 的 tags 键）",
			legacy: map[string]any{"tags": []QuestionTagDict{{
				Code: "T1", CreatedAt: "2026-09-13T10:00:00.000000+08:00", Description: "标签",
				ID: 3, Name: "难点", QuestionCount: int64Ptr(5), SortOrder: 1, Status: 1,
				UpdatedAt: "2026-09-13T10:00:00.000000+08:00",
			}}},
			dto: &QuestionTagListDTO{Tags: []QuestionTagDict{{
				Code: "T1", CreatedAt: "2026-09-13T10:00:00.000000+08:00", Description: "标签",
				ID: 3, Name: "难点", QuestionCount: int64Ptr(5), SortOrder: 1, Status: 1,
				UpdatedAt: "2026-09-13T10:00:00.000000+08:00",
			}}},
		},
		{
			name: "QuestionTagListDTO（question_count 缺省时整个 key 不出现）",
			legacy: map[string]any{"tags": []QuestionTagDict{{
				Code: "T1", ID: 3, Name: "难点",
			}}},
			dto: &QuestionTagListDTO{Tags: []QuestionTagDict{{Code: "T1", ID: 3, Name: "难点"}}},
		},
		{
			name: "CertificateTemplateListDTO（GET /admin/certificate-templates：原 gin.H 的 certificate_templates 键）",
			legacy: map[string]any{"certificate_templates": []CertificateTemplateDict{{
				Code: "C1", CreatedAt: "2026-09-13T10:00:00.000000+08:00", Description: "模板",
				ID: 2, Name: "特种作业证", Status: 1, TemplateURL: "/uploads/t.png",
				UpdatedAt: "2026-09-13T10:00:00.000000+08:00", ValidityDays: 365,
			}}},
			dto: &CertificateTemplateListDTO{CertificateTemplates: []CertificateTemplateDict{{
				Code: "C1", CreatedAt: "2026-09-13T10:00:00.000000+08:00", Description: "模板",
				ID: 2, Name: "特种作业证", Status: 1, TemplateURL: "/uploads/t.png",
				UpdatedAt: "2026-09-13T10:00:00.000000+08:00", ValidityDays: 365,
			}}},
		},
		{
			name: "CredentialListDTO（GET /credentials 与 /admin/credentials：原 gin.H 的 credentials 键）",
			legacy: map[string]any{"credentials": []CredentialDict{{
				Category: "special_operation", Code: "N1", CreatedAt: "2026-09-13T10:00:00.000000+08:00",
				Description: "叉车证", ID: 1, Level: intPtr(1), Name: "叉车司机", SortOrder: 1,
				Status: 1, UpdatedAt: "2026-09-13T10:00:00.000000+08:00",
			}}},
			dto: &CredentialListDTO{Credentials: []CredentialDict{{
				Category: "special_operation", Code: "N1", CreatedAt: "2026-09-13T10:00:00.000000+08:00",
				Description: "叉车证", ID: 1, Level: intPtr(1), Name: "叉车司机", SortOrder: 1,
				Status: 1, UpdatedAt: "2026-09-13T10:00:00.000000+08:00",
			}}},
		},
		{
			name:   "CurrentCredentialDTO（未选证件：key 在、值为 null）",
			legacy: map[string]any{"credential": nil},
			dto:    &CurrentCredentialDTO{},
		},
		{
			name: "SpecialtyListDTO（GET /admin/specialties：原 handler 内联 gin.H 的 specialties 键）",
			legacy: map[string]any{"specialties": []SpecialtyDict{{
				Code: "operation", CreatedAt: "2026-09-16T10:00:00.000000+08:00", Description: "操作",
				Name: "操作", SortOrder: 1, SpecialtyID: 1, Status: 1,
			}}},
			dto: &SpecialtyListDTO{Specialties: []SpecialtyDict{{
				Code: "operation", CreatedAt: "2026-09-16T10:00:00.000000+08:00", Description: "操作",
				Name: "操作", SortOrder: 1, SpecialtyID: 1, Status: 1,
			}}},
		},
		{
			name: "PositionListDTO（GET /positions 与 /admin/positions：原 handler 内联 gin.H 的 positions 键）",
			legacy: map[string]any{"positions": []PositionDict{{
				Code: "driver", CreatedAt: "2026-09-16T10:00:00.000000+08:00", Description: "叉车司机",
				Name: "叉车司机", PositionID: 4, SortOrder: 2, Status: 1,
			}}},
			dto: &PositionListDTO{Positions: []PositionDict{{
				Code: "driver", CreatedAt: "2026-09-16T10:00:00.000000+08:00", Description: "叉车司机",
				Name: "叉车司机", PositionID: 4, SortOrder: 2, Status: 1,
			}}},
		},
		{
			name: "CurrentCredentialDTO（已选证件 / PATCH 切换：同一形状）",
			legacy: map[string]any{"credential": &CredentialDict{
				Category: "skill_level", Code: "S3", ID: 7, Level: intPtr(3), Name: "高级",
			}},
			dto: &CurrentCredentialDTO{Credential: &CredentialDict{
				Category: "skill_level", Code: "S3", ID: 7, Level: intPtr(3), Name: "高级",
			}},
		},
		{
			name: "GroupedCredentialsDTO（GET /credentials/grouped：两个 key 恒在，map 序按 key 排序）",
			legacy: map[string][]CredentialDict{
				"special_operation": {{ID: 1, Category: "special_operation", Name: "叉车司机"}},
				"skill_level":       {{ID: 7, Category: "skill_level", Name: "高级", Level: intPtr(3)}},
			},
			dto: &GroupedCredentialsDTO{
				SkillLevel:       []CredentialDict{{ID: 7, Category: "skill_level", Name: "高级", Level: intPtr(3)}},
				SpecialOperation: []CredentialDict{{ID: 1, Category: "special_operation", Name: "叉车司机"}},
			},
		},
		{
			name: "GroupedCredentialsDTO（空列表：两个 key 的值都是 [] 而不是 null）",
			legacy: map[string][]CredentialDict{
				"special_operation": {},
				"skill_level":       {},
			},
			dto: &GroupedCredentialsDTO{
				SkillLevel:       []CredentialDict{},
				SpecialOperation: []CredentialDict{},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			want, err := json.Marshal(tc.legacy)
			if err != nil {
				t.Fatalf("marshal legacy: %v", err)
			}
			got, err := json.Marshal(tc.dto)
			if err != nil {
				t.Fatalf("marshal dto: %v", err)
			}
			if string(want) != string(got) {
				t.Fatalf("字节不一致（字段顺序 / omitempty / 空对象语义漂移）：\nmap   = %s\nstruct= %s", want, got)
			}
		})
	}
}
