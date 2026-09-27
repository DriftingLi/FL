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
// 反面声明。锁分两处（判据逐条与实现同一文件，行号即证据）：
//
//	(a) 每个 `fact:` tag 的 key 必须登记在表里          —— consumption_fact_lock_test.go
//	(b) 每个 key 至少两个载体，且**同时**有 ≥1 个明文面载体与 ≥1 个投影位；一句错误不能同时
//	    是两件事的身份；而且每个明文面载体必须是 internal/service 里一枚**具名**哨兵
//	    （`errors.New("…")` / `fmt.Errorf("…")` 的字面量）并**真被接进** api 层某个端点的错误面
//	    ——「可达」到这里才不是形容词
//	                                  —— checkFactTable + checkSentinelFaces（同上文件）
//	(c) 表里登记的投影位与代码里真带该 tag 的字段**双向逐字相等**；同一事实的各个投影位
//	    必须**同名**（「同键」的字面意思，字符串包含那条推不出来）—— 同 (b) 的表 + 主测
//	(d) 每个明文面载体的对外句子必须逐字出现在**每一个**同 key 投影位的 swagger description 里
//	    ——把决策 5 的「同措辞」变成机器判据；方向是「真值 → 散文」，不是拿散文当真值（ADR-0047 §1）
//	(e) 扫描面里一处 tag 都没有 ⇒ Fatal（防 tag 改名后锁静默变绿）—— 两个文件各判一次，
//	    失效方式不同，见各自注释
//	(f) 投影位所在类型必须出现在某个 **2xx 响应**的类型闭包里 —— internal/apitypes/
//	    fact_reachability_lock_test.go（闭包与 AST 遍历的只有一份实现；那一侧的射程是六个包，
//	    与这里同一组，两边各有一条「论域里出现了却没扫到 ⇒ 红」的自检，不靠人记）
//
// (b)(c) 里今天**没有真实行能触发**的那几条（重复 key、投影位跨 key 复用、一句错误挂两个 key、
// 投影位不同名、载体查不到宿主、宿主没接进错误面）由 `TestConsumptionFactRulesFire` 与
// `TestSentinelFaceRulesFire` 拿夹具逐条打红一次——不然它们只是几段看起来像断言的代码。
//
// **这张表锁不住什么，写清楚**：
//   - 不登记端点。「事实出现在哪个端点」是可达性问题，真要锁得另建「端点 × 事实」表并配它自己的
//     反向例；在这里顺带抄一份没人核对的端点清单，与批③ 那张从不被查询的 `allowedSameText`
//     是同一件东西（一个从不被查询的清单比没有清单更坏）。
//   - 看不见前端。`frontend/src/utils/contactRequestStatus.ts` 里那句
//     「企业账号已停用或已注销，联系方式已收回」是这一事实的**第三个**消费点，它由自己的
//     vitest 用例钉住（`statusWords.spec.ts`），与本表只靠注释互指。把它纳进来需要一套
//     跨语言的 key 登记，那是另一件事，不在本表里假装已覆盖。
//   - 拦不住「**协同改键名**」。把两格 json 键一起改成别的、登记表跟着改、契约描述不动——
//     (a)–(f) 全绿，因为本锁锁的是「同一事实的各载体互相一致」，不是「这个键名对外承诺过」。
//     键名的稳定性由另外两样东西守：Web 侧靠 `internal/apitypes/codegen_test.go` 的生成物全等
//     比对（改键必然让 committed 的 TS 与注解渲染结果不符 ⇒ 红），移动端靠跨端契约（它手写
//     `.uts` 类型、不吃 codegen，所以本仓没有任何锁能替它发现——只能靠 #1265 那条同步流程）。
//   - 只证到「这枚哨兵接在 api 层的错误面上」，证不到「那句话这次真发出去了」。判据 (b) 会推导
//     宿主标识符并确认它出现在某处 `errors.Is` 或某张 `errStatusTable` 里，但接线不等于可达；
//     那一半归 `contact_company_disable_contract_test.go` 的 HTTP 断言（它要求 403 正文逐字带着
//     这句话）。两半谁都替不了谁——所以登记表认领一个错误载体时，必须同时有一条 HTTP 级断言钉它。
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

// ConsumptionFacts 返回登记表。导出这一件不是为了给别人用——它至今只有测试在读——而是为了
// 让 CI 的 `unused` 认它活着：表本身是小写的，只被 _test.go 读就会被 backend-lint 判死
// （第十五波在测试夹具上实测过这一条）。`Envelopes()` 是同一个形状的先例，也只被自己的测试读。
func ConsumptionFacts() []FactSpec { return consumptionFacts }
