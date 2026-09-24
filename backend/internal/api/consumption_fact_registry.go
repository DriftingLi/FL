// Package api 实现 HTTP handlers。
// 本文件：**消费点对齐登记表**（ADR-0065 决策 8）。
//
// 要锁的东西：一个事实常常有两个形状的载体——
//
//	明文面：一条错误（errors.Is 分得开、对外是一句话），只在这次请求失败时才出现；
//	投影位：一个响应字段（键 + 值），在列表/详情里常驻。
//
// 第十五波把「一个事实只在一处成立」收到了错误面，但**同一事实跨这两种形状同时出现**时，
// 本仓今天没有任何一处机器读得懂「这两格说的是一件事」。`company_disabled` 就是这么来的：
// 它的载体是 `ErrCompanyUnavailable`，两格分别长在两个 DTO 上，三处靠人写注释互指
// （「与联系面明文位置**同键同措辞**的那一格」）——注释会漂，措辞会各改各的，而漂移的代价
// 由消费方付：它按 key 名写判断、按 message 文案做提示，两边一旦不同名不同句，同一件事就
// 又裂成两件。
//
// 所以这里登记「事实 key → 它的全部载体」，字段的 `fact:"<key>"` struct tag 是投影位那一侧的
// 反面声明。锁（consumption_fact_lock_test.go）管五件事：
//
//	(a) 每个 `fact:` tag 的 key 必须登记在表里；
//	(b) 每个 key 至少两个载体，且**同时**有 ≥1 个错误载体与 ≥1 个投影位——只有一侧就不是「对齐」；
//	(c) 表里登记的投影位与代码里真带该 tag 的字段**两边逐字相等**（多登记、少登记都红），
//	    且每个投影位所在类型必须真的出现在某个 2xx 响应闭包里（不在响应里的 tag 是幽灵声明）；
//	(d) 每个错误载体的对外句子必须逐字出现在**每一个**同 key 投影位的 swagger description 里
//	    ——把决策 5 的「同措辞」变成机器判据；方向是「真值 → 散文」，不是拿散文当真值（ADR-0047 §1）；
//	(e) 每个登记的 key 都必须被遍历到 ≥1 个带 tag 的字段，否则 Fatal（防空转）；
//	    整张 tag 扫描一条都没扫到时也 Fatal（防 tag 改名/换写法后锁静默变绿）。
//
// **这张表锁不住什么，写清楚**：它不登记端点。`RecruitResumeCard` 与 `ContactRequestDTO`
// 都被列表/详情两类端点投影，「事实出现在哪个端点」是**可达性**问题而不是**身份**问题，
// 真要锁得另建一张「端点 × 事实」表并给它自己的反向例——那是一件独立的事，不在本表里
// 顺带声明一个没人核对的端点清单（一个从不被查询的清单比没有清单更坏，批③ 的
// `allowedSameText` 就是前车之鉴）。
package api

import (
	"forklift-training/internal/service"
)

// FactSpec 一条「同一事实的多个消费点」登记。
type FactSpec struct {
	// Key 事实名，snake_case，写在响应字段的 `fact:"…"` tag 上。全表唯一。
	Key string
	// Sentinels 该事实的**错误面**载体。判据 (b) 要求非空：一个事实如果只在明文里说得出，
	// 它就没有跨消费点对齐可言，也不该出现在这张表里。
	Sentinels []error
	// Projections 该事实的**投影位**，格式 `包名.类型名.json键`，必须与代码里真带
	// `fact:"Key"` tag 的那批字段逐字相等（判据 (c) 双向）。
	Projections []string
}

// consumptionFacts 消费点对齐登记表。
//
// 只有一行是**诚实**的：本波只把 `company_unavailable` 这一对纳入了机器锁。表里加一行的成本
// 远低于给一个事实找出两个真载体的成本，所以宁可小。
var consumptionFacts = []FactSpec{
	{
		Key: "company_unavailable",
		// 「企业被处置（禁用或注销）」这一件事实，今天有三个载体：
		//   错误面 —— 被禁用的企业自己去取学员明文时的那句 403；
		//   投影位 —— 学员侧交换申请列表的那一格，与招聘者简历卡（列表 + 详情共用一个装配点）的那一格。
		// 后两格必须同名：消费方读的是同一个 key，不是一行两处各起一名。
		Sentinels: []error{service.ErrCompanyUnavailable},
		Projections: []string{
			"service.ContactRequestDTO.company_disabled",
			"service.RecruitResumeCard.company_disabled",
		},
	},
}

// ConsumptionFacts 返回登记表（只读）。导出是为了让这张表在**非测试**代码里也算被引用——
// 一份额外的死清单比没有清单更坏，而 unexported 且只被测试读的表会被 CI 的 unused 判死
// （先例见 envelopeRegistry 的 Envelopes()）。
func ConsumptionFacts() []FactSpec { return consumptionFacts }
