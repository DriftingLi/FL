// 内容精选「发布时间 / 定时发布」的纯函数判据（#1748）：行 → 三态、表单 → 请求体、提示文案。
//
// 只断规则，不挂整页：编辑器里已有「子集提示」组件的同类先例（FeaturedSubsetHint.spec.ts），
// 而这里验的是「一保存会不会把定时取消」这类**语义**，点 UI 反而不容易覆盖全组合。
import { describe, expect, it } from 'vitest'
import {
  FEATURED_STATE_META,
  buildFeaturedPayload,
  describePublishTime,
  editorStatusOf,
  featuredRowState,
  featuredRowTime,
  formatDuration,
  parseIsoToDate,
  toRfc3339,
  type FeaturedFormValues
} from '../featuredSchedule'

const baseForm = (over: Partial<FeaturedFormValues> = {}): FeaturedFormValues => ({
  title: '  标题  ',
  category: 'industry',
  source: '来源',
  summary: '摘要',
  cover_image: '/static/uploads/featured/a.webp',
  content: '正文',
  sort_order: 0,
  status: 0,
  published_at: null,
  ...over
})

describe('featuredRowState', () => {
  it('status=1 是已发布，与有无排期无关', () => {
    expect(featuredRowState({ status: 1, scheduled_at: null })).toBe('published')
    // 脏数据（已发布却仍挂着排期时间）也按已发布读：不变量由库层 CHECK 兜，前端不另立解释
    const dirty = { status: 1, scheduled_at: '2026-10-22T09:00:00+08:00' }
    expect(featuredRowState(dirty)).toBe('published')
  })

  it('草稿 + 有排期 = 定时（不是 status 的取值，而是组合）', () => {
    expect(featuredRowState({ status: 0, scheduled_at: '2026-10-22T09:00:00+08:00' })).toBe('scheduled')
  })

  it('草稿 + 无排期 = 草稿', () => {
    expect(featuredRowState({ status: 0, scheduled_at: null })).toBe('draft')
    expect(featuredRowState({ status: 0 })).toBe('draft')
  })

  it('三态各有唯一文案与语义色', () => {
    expect(FEATURED_STATE_META.draft.label).toBe('草稿')
    expect(FEATURED_STATE_META.scheduled.label).toBe('定时')
    expect(FEATURED_STATE_META.published.label).toBe('已发布')
    expect(new Set(Object.values(FEATURED_STATE_META).map(m => m.tone)).size).toBe(3)
  })
})

describe('featuredRowTime', () => {
  it('定时行显示排期时间，已发布显示发布时间，草稿留空', () => {
    const scheduled = { status: 0, scheduled_at: '2026-10-22T09:00:00+08:00', published_at: null }
    const published = { status: 1, scheduled_at: null, published_at: '2026-10-20T09:00:00+08:00' }
    expect(featuredRowTime(scheduled)).toBe('2026-10-22T09:00:00+08:00')
    expect(featuredRowTime(published)).toBe('2026-10-20T09:00:00+08:00')
    expect(featuredRowTime({ status: 0, scheduled_at: null, published_at: null })).toBeNull()
  })
})

describe('toRfc3339 / parseIsoToDate', () => {
  it('编出来的是带显式偏移的 RFC3339，且与输入是同一时刻', () => {
    const d = new Date(Date.UTC(2026, 9, 21, 7, 30, 0))
    const encoded = toRfc3339(d)
    expect(encoded).toMatch(/^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}[+-]\d{2}:\d{2}$/)
    expect(new Date(encoded).getTime()).toBe(d.getTime())
  })

  it('后端 6 位微秒能解析回同一毫秒', () => {
    const parsed = parseIsoToDate('2026-10-21T15:00:00.123456+08:00')
    expect(parsed).not.toBeNull()
    expect(parsed!.toISOString()).toBe('2026-10-21T07:00:00.123Z')
  })

  it('空值与非法串一律 null（表单的「留空」）', () => {
    expect(parseIsoToDate(null)).toBeNull()
    expect(parseIsoToDate('')).toBeNull()
    expect(parseIsoToDate(undefined)).toBeNull()
    expect(parseIsoToDate('not-a-date')).toBeNull()
  })

  it('往返一次时刻不变', () => {
    const d = new Date(2026, 9, 22, 9, 0, 0)
    expect(parseIsoToDate(toRfc3339(d))!.getTime()).toBe(d.getTime())
  })
})

describe('editorStatusOf（回填规则：定时项在库里是草稿）', () => {
  it('定时项显示为「发布 + 排期时间」，而不是草稿', () => {
    const got = editorStatusOf({ status: 0, scheduled_at: '2026-10-22T09:00:00+08:00', published_at: null })
    expect(got.status).toBe(1)
    expect(got.publishedAt?.toISOString()).toBe('2026-10-22T01:00:00.000Z')
  })

  it('已发布项预填已发布时间（避免一保存就改成现在）', () => {
    const got = editorStatusOf({ status: 1, scheduled_at: null, published_at: '2026-10-20T09:00:00.000000+08:00' })
    expect(got.status).toBe(1)
    expect(got.publishedAt?.toISOString()).toBe('2026-10-20T01:00:00.000Z')
  })

  it('草稿不预填残留时间（那是历史残留，不是此刻意图）', () => {
    const got = editorStatusOf({ status: 0, scheduled_at: null, published_at: '2026-10-01T09:00:00+08:00' })
    expect(got.status).toBe(0)
    expect(got.publishedAt).toBeNull()
  })
})

describe('buildFeaturedPayload（提交规则）', () => {
  it('草稿不带 published_at 键（带上会被后端判 400：草稿不排期）', () => {
    const payload = buildFeaturedPayload(baseForm({ status: 0, published_at: new Date() }))
    expect('published_at' in payload).toBe(false)
    expect(payload.status).toBe(0)
  })

  it('发布 + 指定时间 = 带该时刻的 RFC3339', () => {
    const at = new Date(Date.UTC(2026, 9, 20, 1, 0, 0))
    const payload = buildFeaturedPayload(baseForm({ status: 1, published_at: at }))
    expect(payload.published_at).toBeDefined()
    expect(new Date(payload.published_at!).getTime()).toBe(at.getTime())
  })

  it('发布 + 留空 = 空串（后端语义：用现在）', () => {
    const payload = buildFeaturedPayload(baseForm({ status: 1, published_at: null }))
    expect(payload.published_at).toBe('')
  })

  it('只有「客户端时钟快了几秒」那一格归成现在，未来时刻一律原样提交', () => {
    const now = new Date(Date.UTC(2026, 9, 21, 7, 0, 0))
    const withinSkew = new Date(now.getTime() + 2000)
    const thirtySecondsAhead = new Date(now.getTime() + 30 * 1000)
    const anHourAhead = new Date(now.getTime() + 3600 * 1000)
    const anHourAgo = new Date(now.getTime() - 3600 * 1000)

    // 点「现在」可能落在未来几秒（本机时钟快）⇒ 当「现在」提交，别落成几秒后的定时
    expect(buildFeaturedPayload(baseForm({ status: 1, published_at: withinSkew }), { now }).published_at).toBe('')
    // 选了「半分钟后」「一小时后」⇒ 必须原样发该时刻：否则到点前对访客可见（违反口径 #1）
    expect(buildFeaturedPayload(baseForm({ status: 1, published_at: thirtySecondsAhead }), { now }).published_at).toBe(
      toRfc3339(thirtySecondsAhead)
    )
    expect(buildFeaturedPayload(baseForm({ status: 1, published_at: anHourAhead }), { now }).published_at).toBe(
      toRfc3339(anHourAhead)
    )
    // 补录（一小时前）原样提交
    expect(buildFeaturedPayload(baseForm({ status: 1, published_at: anHourAgo }), { now }).published_at).toBe(
      toRfc3339(anHourAgo)
    )
  })

  it('定时行回填 → 提交：state 变「发布 + 该未来时刻」，请求体仍带该时刻（往返不丢排期）', () => {
    const futureIso = '2026-10-22T09:00:00.000000+08:00'
    const editor = editorStatusOf({ status: 0, scheduled_at: futureIso, published_at: null })
    expect(editor.status).toBe(1)
    const payload = buildFeaturedPayload(
      baseForm({ status: editor.status, published_at: editor.publishedAt }),
      { now: new Date('2026-10-21T15:00:00+08:00') }
    )
    expect(new Date(payload.published_at!).getTime()).toBe(new Date(futureIso).getTime())
  })

  it('标题去首尾空白，正文可用调用方传入的最新值', () => {
    const payload = buildFeaturedPayload(baseForm({ content: '旧正文' }), { content: '新正文' })
    expect(payload.title).toBe('标题')
    expect(payload.content).toBe('新正文')
  })
})

describe('formatDuration / describePublishTime', () => {
  it('时长按分钟级口径说人话', () => {
    expect(formatDuration(90 * 60 * 1000)).toBe('1 小时 30 分钟')
    expect(formatDuration(30 * 60 * 60 * 1000)).toBe('1 天 6 小时')
    // 向上取整到分钟：10 秒后到点说的是「1 分钟」，比「不到 1 分钟」更接近实际等待
    expect(formatDuration(10 * 1000)).toBe('1 分钟')
    expect(formatDuration(0)).toBe('不到 1 分钟')
  })

  it('留空 / 过去 / 未来三种提示互不相同，且未来那句带时长', () => {
    const now = new Date('2026-10-21T15:00:00+08:00')
    expect(describePublishTime(null, now)).toContain('立即发布')
    expect(describePublishTime(new Date('2026-10-20T15:00:00+08:00'), now)).toContain('补录')
    const future = describePublishTime(new Date('2026-10-21T17:00:00+08:00'), now)
    expect(future).toContain('2 小时')
    expect(future).toContain('自动发布')
  })
})