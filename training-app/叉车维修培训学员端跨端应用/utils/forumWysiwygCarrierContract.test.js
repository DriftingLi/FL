/**
 * 发帖载体接入 WYSIWYG 契约（#2641）。
 *
 * 全部是**源码接线锁**（.uvue / .uts 不可 jest import，沿用仓内同款缝）；它不构成 ③ 门证据，
 * 只守「实现面不回潮」——真判据是票面 AC 与真机（#2644）。
 */
const path = require('path')
const { readText } = require('./utsHarness')

const ROOT = path.join(__dirname, '..')
const read = (rel) => readText(path.join(ROOT, rel))

const PAGE_REL = 'pages/forum/forum-create.uvue'
const FORM_REL = 'pages/forum/composables/useForumCreateForm.uts'
const WYSIWYG_REL = 'components/forum-wysiwyg-input.uvue'

const page = read(PAGE_REL)
const form = read(FORM_REL)
const wysiwyg = read(WYSIWYG_REL)

describe('AC① 新发帖：一律 markdown 落库，输入形态只剩一种', () => {
  it('新建载荷末位恒为 FORMAT_MARKDOWN（content 是编辑器序列化产物）', () => {
    expect(form).toMatch(
      /await createForumTopicApi\(title\.value\.trim\(\), content\.value\.trim\(\), images\.value, selectedCategory\.value, FORMAT_MARKDOWN\)/
    )
    expect(form).not.toContain('authorFormat')
  })

  it('作者格式偏好彻底退出：页面与表单域都不再读写记忆位', () => {
    for (const src of [page, form]) {
      expect(src).not.toContain('readAuthorFormat')
      expect(src).not.toContain('writeAuthorFormat')
      expect(src).not.toContain('normalizeAuthorFormat')
    }
    // 档位胶囊不再出现（旧组件仍收该 prop，但恒传 false）
    expect(page).toContain(':show-format-switch="false"')
    expect(page).not.toContain('showFormatSwitch')
  })

  it('能力端接的是 WYSIWYG 组件，且只经 update:markdown / update:issues 收结果', () => {
    expect(page).toContain("import ForumWysiwygInput from '../../components/forum-wysiwyg-input.uvue'")
    expect(page).toContain('@update:markdown="onMarkdownChange"')
    expect(page).toContain('@update:issues="onIssuesChange"')
    expect(page).toContain('v-if="useRichEditor"')
  })
})

describe('AC② 编辑旧帖：按原格式回填，PUT 不带 content_format', () => {
  it('回填读详情 DTO 的 content_format，编辑态仍走旧组件（无 markdown→delta 反向解析）', () => {
    expect(form).toContain('editFormat.value = data.content_format')
    expect(form).toMatch(/return isEdit\.value \? editFormat\.value : FORMAT_MARKDOWN/)
    expect(page).toContain(':format="effectiveFormat"')
  })

  it('PUT 载荷不带 content_format（编辑不变式）', () => {
    expect(form).toMatch(
      /await updateForumTopicApi\(editTopicId\.value, title\.value\.trim\(\), content\.value\.trim\(\), images\.value, selectedCategory\.value\)/
    )
    expect(form).not.toMatch(/updateForumTopicApi\([^)]*content_format/)
  })
})

describe('AC③ 发布前保真检查：提交按钮附近持续可见 + 空正文禁用', () => {
  it('保真提示行在提交按钮**之前**渲染（同一 .submit-section 内）', () => {
    const section = page.slice(
      page.indexOf('<view v-if="!isResourceMode && !isFeedbackMode" class="submit-section">')
    )
    const notice = section.indexOf('<text v-if="fidelityNotice.length > 0" class="fidelity-notice">')
    const button = section.indexOf('class="submit-btn"')
    expect(notice).toBeGreaterThan(-1)
    expect(button).toBeGreaterThan(notice)
  })

  it('组件内那行由宿主关掉，两处只出现一次', () => {
    expect(page).toContain(':show-fidelity-notice="false"')
    expect(wysiwyg).toContain('showFidelityNotice && fidelityNotice.length > 0')
  })

  it('序列化结果空 / 纯空白 ⇒ 提交按钮禁用', () => {
    expect(form).toMatch(/if \(content\.value\.trim\(\)\.length == 0\) return false/)
    expect(page).toContain(':class="{ disabled: !canSubmit }"')
  })

  it('保真提示来自序列化器的 issues（不是页面自己猜）', () => {
    expect(form).toContain('richFidelityNotice(richIssues.value)')
    expect(form).toContain('richIssues.value = value')
  })
})

describe('AC⑤ 平台例外：无 editor 端仍走记号工具栏', () => {
  it('页面用编译期 #ifndef 摘掉 WYSIWYG 分支，旧组件兜底', () => {
    expect(page).toContain('<!-- #ifndef MP-ALIPAY || MP-BAIDU || MP-TOUTIAO -->')
    expect(page).toContain('<!-- #endif -->')
    expect(page).toContain('<ForumMarkdownInput v-if="!useRichEditor"')
  })

  it('能力判定落在 composable 的 #ifdef 块里', () => {
    expect(form).toContain('// #ifdef MP-ALIPAY || MP-BAIDU || MP-TOUTIAO')
    expect(form).toContain('richEditorSupported = false')
    expect(form).toMatch(/const useRichEditor = computed<boolean>/)
  })
})

describe('页面壳层零自持状态（T2 #2640 第 8 条验收）', () => {
  it('页面不 import ref / computed，也不自持响应式状态', () => {
    expect(page).not.toMatch(/import \{[^}]*\bref\b[^}]*\} from 'vue'/)
    expect(page).not.toMatch(/import \{[^}]*\bcomputed\b[^}]*\} from 'vue'/)
    expect(page).not.toMatch(/\bref</)
    expect(page).not.toMatch(/\bcomputed</)
    expect(page).not.toMatch(/\breactive\(/)
  })

  it('表单域全部经 composable 的显式结果类型消费', () => {
    expect(page).toContain('const form = useForumCreateForm()')
    expect(form).toContain('export type UseForumCreateFormResult')
    expect(form).toContain('export function useForumCreateForm() : UseForumCreateFormResult')
  })
})
