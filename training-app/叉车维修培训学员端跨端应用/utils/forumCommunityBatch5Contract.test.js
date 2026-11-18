/**
 * 社区批（B5 #2521）契约：G-08 / G-09 / N2 #2549 形状 + 签到贴片 580:6795 全状态。
 *
 * 全部是**源码接线锁**（.uvue / .uts 不可 jest import，沿用仓内同款缝）；它不构成 ③ 门证据，
 * 只守「实现面不回潮」——真判据是逐页截图对照与真实 API（票面验收）。
 */
const path = require('path')
const { readText } = require('./utsHarness')

const ROOT = path.join(__dirname, '..')
const read = (rel) => readText(path.join(ROOT, rel))

const FORUM = 'pages/forum/forum.uvue'
const TAB_BAR = 'pages/forum/components/forum-tab-bar.uvue'
const FEED = 'composables/useTopicFeed.uts'
const API = 'api/forum.uts'
const CREATE = 'pages/forum/forum-create.uvue'
/** #2641：发帖页表单域整域搬进 composable（页面壳层零自持状态），模式 / 提交判据随之落在这里 */
const CREATE_FORM = 'pages/forum/composables/useForumCreateForm.uts'
const DETAIL = 'pages/forum/forum-detail.uvue'
const CHECKIN = 'pages/forum/check-in.uvue'

describe('forum 四 Tab 与 N2 #2549 experience 参数', () => {
  const tabBar = read(TAB_BAR)
  const feed = read(FEED)
  const api = read(API)

  it('Tab Bar 四 Tab 在场（广场 / 资源 / 知识问答 / 备考经验），经验走 onSwitch(experience)', () => {
    for (const label of ['广场', '资源', '知识问答', '备考经验']) {
      expect(tabBar).toContain(label)
    }
    expect(tabBar).toMatch(/@click="onSwitch\('experience'\)"/)
  })

  it('经验 Tab 用 experience=true（N2 #2549 形状），不发 category=experience', () => {
    expect(feed).toMatch(/currentTab\.value === 'experience'[\s\S]*?experience = 'true'/)
    expect(feed).not.toMatch(/category = 'experience'/)
  })

  it('API 按 experience 参数名透传（不是 is_experience）', () => {
    expect(api).toContain("params['experience'] = experience")
    expect(api).not.toContain("params['is_experience']")
  })
})

describe('G-08：发帖模块只留讨论 / 提问两值', () => {
  const form = read(CREATE_FORM)

  it('模块行两值：讨论 / 提问；资源与备考经验都不占写侧类别位', () => {
    expect(form).toContain("{ label: '讨论', value: 'discussion' }")
    expect(form).toContain("{ label: '提问', value: 'question' }")
    expect(form).not.toContain("{ label: '资源', value: 'resource' }")
    expect(form).not.toContain("{ label: '备考经验', value: 'experience' }")
  })

  it('资源仍是 URL scope=resource 的真模式（投稿表单不流入发帖接口）', () => {
    expect(form).toMatch(/if \(s == 'resource'\) \{[\s\S]*?resourceMode\.value = true/)
    expect(form).toMatch(/if \(isResourceMode\.value\) return/)
  })
})

describe('G-09：反馈改单向提交，需求详情屏不实现', () => {
  const create = read(CREATE)
  const form = read(CREATE_FORM)
  const detail = read(DETAIL)

  it('forum-create 有 scope=feedback 模式，且走现有 POST /feedback（submitFeedbackApi）', () => {
    expect(form).toMatch(/else if \(s == 'feedback'\) \{[\s\S]*?feedbackMode\.value = true/)
    expect(form).toContain('submitFeedbackApi')
    expect(form).toContain("import { submitFeedbackApi, FEEDBACK_CONTENT_MAX } from '../../../api/feedback'")
  })

  it('反馈无 type / 无工单号 / 无回复线程：提交载荷只有正文 + 图 + 联系方式', () => {
    expect(form).toMatch(/submitFeedbackApi\(text, feedbackNoImages, feedbackContact\.value\.trim\(\)\)/)
    for (const banned of ['feedbackType', 'ticketNo', 'workOrder', 'replyThread']) {
      expect(form).not.toContain(banned)
    }
  })

  it('提交后只显示「已提交」态（feedbackSubmitted 收口），不做列表 / 状态流转', () => {
    expect(create).toContain('v-if="feedbackSubmitted"')
    expect(create).toContain('已提交')
    expect(create).not.toContain('listFeedbackApi')
  })

  it('需求详情屏与原型需求文案不回潮', () => {
    expect(detail).not.toContain('scope=feedback')
    expect(detail).not.toContain('需求详情')
    for (const banned of ['提交建议', '一个需求只提一个功能点', '先搜索类似需求']) {
      expect(create).not.toContain(banned)
      expect(detail).not.toContain(banned)
    }
  })
})

describe('签到贴片 580:6795：五态瓦片与日号显示', () => {
  const checkin = read(CHECKIN)

  it('五态 class 全在：连续签到 / 打卡完成 / 未打卡成功 / 未到 / 默认', () => {
    for (const cls of ['.cell-streak', '.cell-checked', '.cell-missed', '.cell-future', '.cell-default']) {
      expect(checkin).toContain(cls)
    }
  })

  it('所有瓦片数字位一律显示日号（连续天数只由黄底+勾表达）', () => {
    expect(checkin).toMatch(/function cellLabel\(cell : CalendarCell\) : string \{\s*return cell\.day\.toString\(\)\s*\}/)
    expect(checkin).not.toMatch(/if \(cell\.isStreak\) return consecutiveDays\.value\.toString\(\)/)
    expect(checkin).toContain('cellLabel(cell)')
  })

  it('打卡完成 / 连续签到共用蓝底白对勾；未到与逾期各用淡 / 灰金币', () => {
    expect(checkin).toContain('badge badge-check')
    expect(checkin).toContain('badge-coin-faded')
    expect(checkin).toContain('badge-coin-missed')
  })
})

describe('补边（NAV_FLOW 裁定 4）：我的出口与个人动态出口', () => {
  it('forum Tab Bar 头像仍接个人动态（485:5624），壳层不另造第二入口', () => {
    expect(read(TAB_BAR)).toContain("emit('personalActivity')")
    expect(read(FORUM)).toContain("uni.navigateTo({ url: '/pages/profile/personal-activity' })")
  })

  it('原生 tabBar 含「我的」出口（pages.json）', () => {
    expect(read('pages.json')).toContain('pages/profile/profile')
  })
})
