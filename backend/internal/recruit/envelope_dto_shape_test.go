// Package recruit 测试：信封 DTO 的字节锁（spec #954 片二里属本域的一例，
// P2 波 4e 随 DTO 从 internal/service/envelope_dto_shape_test.go 搬来）。
//
// 左边是**改造前的 map 形态**（不是手抄的 JSON 字面量，否则抄错即与 DTO 同错），
// 右边是收口后的 DTO：encoding/json 对 map 按 key 排序输出、对 struct 按字段声明序输出，
// 所以字段顺序写错、加了 omitempty、或把空值写成 nil，这里立刻红。
package recruit

import (
	"encoding/json"
	"testing"
)

func TestRecruitDomainEnvelopeDTOBytes(t *testing.T) {
	cases := []struct {
		name   string
		legacy any
		dto    any
	}{
		{
			name:   "RecruitMeDTO（GET /api/recruit/me，原裸 handler）",
			legacy: map[string]any{"user_id": 9, "account": "hr009", "role": "recruiter"},
			dto:    &RecruitMeDTO{UserID: 9, Account: "hr009", Role: "recruiter"},
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
