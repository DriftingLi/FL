// 生成文件，勿手改（ADR-0019 契约 codegen 专项第一步 / spec #940 片五③）。
// 域：每日打卡（/api/check-in/*，ADR-0028 独立蓝图）
// 唯一事实源：后端注解 → backend/docs/swagger.json（CI 有新鲜度锁：backend-lint 的 swagger 步骤）。
// 再生成：cd backend && go run ./cmd/gen-apitypes
// 同步契约：backend/internal/apitypes/codegen_test.go 把本文件与注解渲染结果全等比对。
//
// 覆盖端点：
//   POST /check-in
//   GET  /check-in/calendar
//   GET  /check-in/rank
//
// 覆盖的 Go 类型：CheckInCalendarResult / CheckInDay / CheckInRankItem / CheckInRankResult / CheckInResult / ForumAuthor
//
// 已知限制（除显式标注 x-nullable 的字段外，一律按非可选渲染）：
//   - 不区分「缺省 / null / 零值」三态（Go 指针与 omitempty 在 swagger 里默认不可见；
//     需要精确可空时给字段加 extensions:"x-nullable"，本生成器会渲染 T | null）；
//   - 不生成 query / body 的入参类型（只生成响应形状）。
// 需要精确可空或入参类型时，先在注解层补齐（见 spec #940 片五②的差集清单）。

export interface CheckInCalendarResult {
  days: CheckInDay[]
  streak: number
  today_checked: boolean
  total: number
}

export interface CheckInDay {
  checked: boolean
  date: string
  points: number
}

export interface CheckInRankItem {
  rank: number
  streak: number
  today_checked: boolean
  total: number
  user: ForumAuthor
}

export interface CheckInRankResult {
  items: CheckInRankItem[]
  me: CheckInRankItem | null
  page: number
  pages: number
  total: number
}

export interface CheckInResult {
  checked: boolean
  points: number
  streak: number
  today_checked: boolean
  total: number
}

export interface ForumAuthor {
  avatar_url: string
  user_id: number
  username: string
}
