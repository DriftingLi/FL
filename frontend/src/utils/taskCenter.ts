// 任务中心分组与状态元数据（#409/#410）。
// #388 前占位页遗留的 mock 任务数据与 localStorage 辅助函数（loadTasks/saveTasks/
// loadPoints/savePoints/resetTaskCenter）已移除——任务中心只信任后端实时任务列表，
// 不再有任何本地占位回退（spec #408「占位数据不再是后端异常的正确呈现」）。
// 分组值域（ADR-0054）：growth 已退役并入 daily —— 三项 growth 任务的
// (daily_limit, total_limit) 与 daily_* 逐字一致，group 本就是 total_limit 的投影。
export type TaskGroup = 'daily' | 'newbie'
export type TaskStatus = 'todo' | 'claimable' | 'claimed'

export interface TaskItem {
  id: number
  group: TaskGroup
  title: string
  desc: string
  points: number
  status: TaskStatus
  progress?: number
  total?: number
}

export interface PointsSummary {
  balance: number
  totalEarned: number
}

export const groupLabelMap: Record<TaskGroup, string> = {
  daily: '每日任务',
  newbie: '新手任务',
}

// 分组口径文案与后端配置同源。#410 后 growth 与 daily 同为「每日可领」；
// ADR-0054 干脆把 growth 并入 daily —— 旧文案「当日达成当日领」也随之收紧：
// daily_login 已无行为前置（进任务中心即可领），不再对全组成立。
export const groupDescMap: Record<TaskGroup, string> = {
  daily: '每日 0 点重置，当日各可领一次',
  newbie: '一次性任务，完成后不再出现',
}

// 今日可得：**只算 daily 组**未领取任务的奖励之和（#2065 / 报告 §8 Q11 第 4 条）。
//
// 为什么不能把所有未领取任务加进来：newbie 是**一次性**任务（total_limit 非空），它的分不是
// 「今天能拿到的」，混进来会让卡片上的「今日可得」永远比真实值大（旧实现 110 vs 真实 60）——
// 这不是文案错，是**口径错**：一处把「还有多少分可拿」与「今天能拿多少分」压成同一个数。
// 分组语义的唯一出处是后端 config 的 group（ADR-0054：daily = 可再生、newbie = 终身一次），
// 这里只按它分档，不在端上另立清单。
//
// 口径前提：现行配置 daily_limit 恒为 1（每个每日任务一天只能领一次），故「任务分 × 1」
// 就是今日可得；若将来出现 daily_limit > 1 的任务，得先把 daily_limit 下放到 DTO
// （今天没有这个事实源，不预支）。
export interface EarnableTask {
  /** 分组：daily = 每日可再生 / newbie = 终身一次（后端 config 的 group 投影） */
  group: string
  status: string
  points: number
}

export function todayEarnable(tasks: EarnableTask[]): number {
  return tasks
    .filter((t) => t.group === 'daily' && t.status !== 'claimed')
    .reduce((sum, t) => sum + t.points, 0)
}

// 幂等错误的语义分级（#409）：按任务分组区分提示文案，不再依赖后端中文字串匹配。
export function claimDupMessage(group: TaskGroup): string {
  return group === 'newbie' ? '已领取' : '今日已领取'
}

// 是否属于「额度已用尽」类幂等失败（用于把任务置为已领取的本地自愈）。
export function isClaimExhausted(_group: TaskGroup, kind: string | undefined, _message: string): boolean {
  // 判定只依赖客户端错误分类（kind === 'business'）：领取接口的 4xx 业务失败只有
  // 「额度已用尽/已领取」语义，无其它业务分支；后端调整文案后自愈逻辑不失效（#409）。
  return kind === 'business'
}
