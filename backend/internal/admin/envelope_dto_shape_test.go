// Package admin 测试：信封 DTO 的字节锁（spec #940 片二/片三里属本域的三例，
// P2 波 4d 随 DTO 从 internal/service/envelope_dto_shape_test.go 搬来）。
//
// 每张表都是「收口前的 map 形态 ↔ 收口后的 typed DTO」：两者的 json.Marshal 结果必须**逐字节相等**。
// 这不是重复断言 —— encoding/json 对 map 按 key 排序输出，而 struct 按字段声明序输出，
// 所以字段顺序写错、加了 omitempty、或把空切片写成 nil，这里立刻红。
// 参照物写法沿用 question_dto_shape_test.go 的 legacy*Dict 先例。
package admin

import (
	"encoding/json"
	"testing"

	"forklift-training/internal/model"
)

func TestAdminDomainEnvelopeDTOBytes(t *testing.T) {
	user := &model.HrwaiUser{ID: 12, UID: 20260012, Account: "hrwai012", Username: "张三", Phone: "13800000001"}
	newUser := NewHrwaiUserCreatedDTO(user)

	cases := []struct {
		name   string
		legacy any
		dto    any
	}{
		{
			name:   "StatusResultDTO（HRWAI 用户 / 导师 / 招聘者三个开关端点共用；来源 int16）",
			legacy: map[string]any{"status": int16(1)},
			dto:    &StatusResultDTO{Status: 1},
		},
		{
			name:   "StatusResultDTO（来源 int，零值也不省略）",
			legacy: map[string]any{"status": 0},
			dto:    &StatusResultDTO{},
		},
		{
			name: "HrwaiUserCreatedDTO（新增 HRWAI 用户 201：password 不入响应，uid 走 FormatUID）",
			legacy: map[string]any{
				// core.FormatUID(user.UID) 的十进制展开（uid.go:36 就是 strconv.FormatInt(uid, 10)）：
				// 域包测试不 import internal/core —— 留驻侧的单点已在留驻侧自己的用例里锁过。
				"id": user.ID, "uid": "20260012", "account": user.Account,
				"username": user.Username, "phone": user.Phone,
			},
			dto: &newUser,
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
