package auth

import (
	"encoding/json"
	"testing"

	"forklift-training/internal/model"
)

// spec #940 片三（含片二）：信封 DTO 的 shape-lock。
//
// 每张表都是「收口前的 map 形态 ↔ 收口后的 typed DTO」：两者的 json.Marshal 结果必须**逐字节相等**。
// 这不是重复断言 —— encoding/json 对 map 按 key 排序输出，而 struct 按字段声明序输出，
// 所以字段顺序写错、加了 omitempty、或把空切片写成 nil，这里立刻红。
// 参照物写法沿用 question_dto_shape_test.go 的 legacy*Dict 先例。
//
// P2 波 3a（ADR-0070）：本用例原住 internal/service/envelope_dto_shape_test.go，
// 随「信封形状用例跟着 DTO 的归属域走」搬进 internal/auth —— 表里只留 auth 域自己的 DTO，
// 留驻 service 的那几条留在原文件，两边共用同一套断言写法。
func TestEnvelopeDTOShapeLock(t *testing.T) {
	cases := []struct {
		name   string
		legacy any
		dto    any
	}{
		{
			name: "TutorRegisterResultDTO",
			legacy: map[string]any{
				"tutor_id": 7,
				"username": "tutor1",
				"name":     "张老师",
			},
			dto: &TutorRegisterResultDTO{Name: "张老师", TutorID: 7, Username: "tutor1"},
		},
		{
			name: "WechatQRCodeInfoDTO",
			legacy: map[string]any{
				"enabled": false,
				"qr_url":  "",
				"message": "微信授权暂未配置，请等待开放平台配置完成后使用",
			},
			dto: &WechatQRCodeInfoDTO{Enabled: false, Message: "微信授权暂未配置，请等待开放平台配置完成后使用", QRURL: ""},
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
				t.Fatalf("字节不一致（字段顺序 / omitempty / 空切片语义漂移）：\nmap   = %s\nstruct= %s", want, got)
			}
		})
	}
}

// spec #954 片二：handler **内联构造**的响应 map 定型为 DTO 后的同一套字节锁。
//
// ADR-0009 当年把「handler 里内联构造的响应 map」明确划出范围（判据一节：另立片）——
// 本片就是那一片，因此沿用同一个机制与同一个参照物：左边是**改造前的 map 形态**
// （不是手抄的 JSON 字面量，否则抄错即与 DTO 同错），右边是收口后的 DTO。
//
// P2 波 3a（ADR-0070）：同 TestEnvelopeDTOShapeLock，随域搬进 internal/auth。
func TestInlineResponseDTOBytes(t *testing.T) {
	rec := &model.RecruiterUser{
		ID: 7, Username: "hr001", CompanyName: "叉车租赁有限公司", CreditCode: "91310000MA1K3XYZ",
		BusinessScope: "叉车租赁与维修", ContactName: "王工", ContactPhone: "13800000000",
		ContactEmail: "hr@example.com", Wechat: "wx_hr001", Status: 1,
	}
	created, updated := NewRecruiterCreatedDTO(rec), NewRecruiterUpdatedDTO(rec)

	cases := []struct {
		name   string
		legacy any
		dto    any
	}{
		{
			name: "RecruiterCreatedDTO（创建 201：含 status）",
			legacy: map[string]any{
				"id": rec.ID, "username": rec.Username, "company_name": rec.CompanyName,
				"credit_code": rec.CreditCode, "business_scope": rec.BusinessScope,
				"contact_name": rec.ContactName, "contact_phone": rec.ContactPhone,
				"contact_email": rec.ContactEmail, "wechat": rec.Wechat, "status": rec.Status,
			},
			dto: &created,
		},
		{
			name: "RecruiterCreatedDTO（零值：无 omitempty，10 个 key 一个不少）",
			legacy: map[string]any{
				"id": 0, "username": "", "company_name": "", "credit_code": "",
				"business_scope": "", "contact_name": "", "contact_phone": "",
				"contact_email": "", "wechat": "", "status": int16(0),
			},
			dto: &RecruiterCreatedDTO{},
		},
		{
			name: "RecruiterUpdatedDTO（编辑 200：与创建同一个投影少一个 status —— 现状差异按字节保留）",
			legacy: map[string]any{
				"id": rec.ID, "username": rec.Username, "company_name": rec.CompanyName,
				"credit_code": rec.CreditCode, "business_scope": rec.BusinessScope,
				"contact_name": rec.ContactName, "contact_phone": rec.ContactPhone,
				"contact_email": rec.ContactEmail, "wechat": rec.Wechat,
			},
			dto: &updated,
		},
		{
			name:   "RecruiterPasswordResetResult（改造前是空 map：data 必须是 {} 而不是 null）",
			legacy: map[string]any{},
			dto:    &RecruiterPasswordResetResult{},
		},
		{
			name:   "RefreshResultDTO（POST /auth/refresh：原 raw handler 内联 map[string]string）",
			legacy: map[string]string{"token": "acc-1", "refresh_token": "ref-1"},
			dto:    &RefreshResultDTO{RefreshToken: "ref-1", Token: "acc-1"},
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
				t.Fatalf("字节不一致（字段顺序 / omitempty / 空切片语义漂移）：\nmap   = %s\nstruct= %s", want, got)
			}
		})
	}
}
