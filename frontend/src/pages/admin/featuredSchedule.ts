// 内容精选「发布时间 / 定时发布」的前端纯函数（#1748，机制与被否备选见 ADR-0076）。
//
// 为什么抽成纯函数并单测：三态徽标、表单 → 请求体、提示文案都是**规则**，不是布局。
// 留在页面里只能靠点 UI 才验得到，而这三件事一旦漂移，症状是「一保存就把定时取消」
// 或「读者看到的时间与编辑填的不是同一刻」—— 两种都不报错，只会悄悄错。

export type FeaturedRowState = 'draft' | 'scheduled' | 'published'

/**
 * 行 → 三态。定时态**不是** status 的取值，而是「草稿 + 有排期时间」，
 * 与后端列表 `state=scheduled` 的过滤条件是同一个组合（status = 0 且 scheduled_at 非空）。
 */
export function featuredRowState(row: { status: number; scheduled_at?: string | null }): FeaturedRowState {
  if (row.status === 1) return 'published'
  return row.scheduled_at ? 'scheduled' : 'draft'
}

export interface FeaturedStateMeta {
  label: string
  tone: 'info' | 'warning' | 'success'
}

export const FEATURED_STATE_META: Record<FeaturedRowState, FeaturedStateMeta> = {
  draft: { label: '草稿', tone: 'info' },
  scheduled: { label: '定时', tone: 'warning' },
  published: { label: '已发布', tone: 'success' }
}

/** 列表「时间」列：定时中显示排期时间，已发布显示发布时间，草稿留空。 */
export function featuredRowTime(row: {
  status: number
  scheduled_at?: string | null
  published_at?: string | null
}): string | null {
  const state = featuredRowState(row)
  if (state === 'scheduled') return row.scheduled_at ?? null
  if (state === 'published') return row.published_at ?? null
  return null
}

/**
 * Date → RFC3339（带显式偏移）。后端严格按 RFC3339 解析，**缺偏移会被判 400**；
 * 这里用当日时区的偏移（东八区机器上即 `+08:00`），传过去的是同一时刻，与时区无关。
 */
export function toRfc3339(d: Date): string {
  const pad = (n: number) => String(Math.abs(n)).padStart(2, '0')
  const wall =
    `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}` +
    `T${pad(d.getHours())}:${pad(d.getMinutes())}:${pad(d.getSeconds())}`
  const offset = -d.getTimezoneOffset() // 分钟数，东为正
  const sign = offset >= 0 ? '+' : '-'
  const abs = Math.abs(offset)
  return `${wall}${sign}${pad(Math.floor(abs / 60))}:${pad(abs % 60)}`
}

/**
 * ISO 串 → Date；空值 / 非法返回 null（表单里 null 就是「留空」）。
 * 后端时间串是 6 位微秒（ADR-0043），而 Date 只认毫秒：先截成 3 位再解析，
 * 避免个别引擎把多余位数读成非法日期。
 */
export function parseIsoToDate(iso?: string | null): Date | null {
  if (!iso) return null
  const normalized = iso.replace(/\.(\d{3})\d+/, '.$1')
  const d = new Date(normalized)
  return isNaN(d.getTime()) ? null : d
}

/**
 * 编辑器回填（关键规则）：定时项在库里是**草稿**（靠保持草稿态实现不可见），
 * 所以必须用「scheduled_at 有无」决定展示，否则一保存就把定时取消。
 * 草稿不预填时间：库里那个 published_at 是历史残留，不是编辑此刻的意图。
 */
export function editorStatusOf(row: {
  status: number
  scheduled_at?: string | null
  published_at?: string | null
}): { status: number; publishedAt: Date | null } {
  if (featuredRowState(row) === 'scheduled') {
    return { status: 1, publishedAt: parseIsoToDate(row.scheduled_at) }
  }
  if (row.status === 1) {
    return { status: 1, publishedAt: parseIsoToDate(row.published_at) }
  }
  return { status: 0, publishedAt: null }
}

export interface FeaturedFormValues {
  title: string
  category: string
  source: string
  summary: string
  cover_image: string
  content: string
  sort_order: number
  status: number
  published_at: Date | null
}

export interface FeaturedPayload {
  title: string
  category: string
  source: string
  summary: string
  cover_image: string
  content: string
  sort_order: number
  status: number
  published_at?: string
}

export interface FeaturedPayloadOptions {
  /** 正文取值（编辑器里可能比 v-model 新，由调用方先从编辑器取一次）。 */
  content?: string
  /** 判定「就是现在」的参照时刻（默认取调用时的 now；测试可注入）。 */
  now?: Date
}

/**
 * 客户端时钟可能比服务器快一点：点「现在」拿到的时刻落到服务端时可能仍在「未来」几秒。
 * 只有**这么窄**、且**只在未来那一侧**的偏差才归成「现在」提交 ——
 * 更宽的容差会把「选了 1 分钟后的整点」也吞成立即发布，那正好违反「到点前对外不可见」。
 */
const NOW_SKEW_MS = 5 * 1000

/**
 * 表单 → 请求体（提交规则）：发布时带时间（空串表示「用现在」）；草稿**不带**时间。
 * 草稿带时间会被后端判 400（草稿不排期），所以这一格不能用「有就带」。
 */
export function buildFeaturedPayload(
  form: FeaturedFormValues,
  options: FeaturedPayloadOptions = {}
): FeaturedPayload {
  const payload: FeaturedPayload = {
    title: form.title.trim(),
    category: form.category,
    source: form.source,
    summary: form.summary,
    cover_image: form.cover_image,
    content: options.content ?? form.content,
    sort_order: form.sort_order,
    status: form.status
  }
  if (form.status === 1) {
    const at = form.published_at
    const nowMs = (options.now ?? new Date()).getTime()
    // 三种填法三种提交（与后端语义矩阵一一对应，不做跨格猜测）：
    //   留空 → 空串（用服务端现在）；过去或远未来 → 原样发该时刻；
    //   仅「客户端时钟快了几秒」那一窄格 → 空串，免得点「现在」落成一条几秒后的定时。
    const aheadMs = at ? at.getTime() - nowMs : 0
    payload.published_at = !at || (aheadMs > 0 && aheadMs <= NOW_SKEW_MS) ? '' : toRfc3339(at)
  }
  return payload
}

/** 剩余时长 → 人话（分钟级口径：#1748 已确认精度 1 分钟）。 */
export function formatDuration(ms: number): string {
  const totalMinutes = Math.max(0, Math.ceil(ms / 60000))
  const days = Math.floor(totalMinutes / 1440)
  const hours = Math.floor((totalMinutes % 1440) / 60)
  const minutes = totalMinutes % 60
  const parts: string[] = []
  if (days > 0) parts.push(`${days} 天`)
  if (hours > 0) parts.push(`${hours} 小时`)
  if (days === 0 && minutes > 0) parts.push(`${minutes} 分钟`)
  return parts.length ? parts.join(' ') : '不到 1 分钟'
}

/**
 * 发布时间 → 提示文案。只说「将」：生效以服务端为准（时钟以服务器业务时区为口径）。
 */
export function describePublishTime(at: Date | null, now: Date = new Date()): string {
  if (!at) return '留空 = 保存后立即发布'
  const aheadMs = at.getTime() - now.getTime()
  // 与提交规则同一格：客户端时钟的那几秒不算「定时」（否则提示说「将在 0 分钟后发布」）
  if (aheadMs > 0 && aheadMs <= NOW_SKEW_MS) return '将在保存后立即发布'
  if (aheadMs <= 0) {
    return '将按该时间补录发布（会改变列表排序与上/下一篇）'
  }
  return `将在 ${formatDuration(aheadMs)} 后自动发布（定时期间对外不可见）`
}
