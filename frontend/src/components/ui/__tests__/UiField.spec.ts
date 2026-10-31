// 字段族（#2156 表单批）：守「无效态位置 + 错误可多条 + 纯增量」三件事。
// 只断言外部可观察行为（属性、文本、节点数），不断言具体 class 取值。
import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import UiField from '../UiField.vue'
import UiFieldLabel from '../UiFieldLabel.vue'
import UiFieldDescription from '../UiFieldDescription.vue'
import UiFieldError from '../UiFieldError.vue'
import UiFieldGroup from '../UiFieldGroup.vue'

describe('UiField（字段容器）', () => {
  it('invalid 时挂 data-invalid，默认不挂', () => {
    const plain = mount(UiField)
    expect(plain.attributes('data-invalid')).toBeUndefined()

    const invalid = mount(UiField, { props: { invalid: true } })
    expect(invalid.attributes('data-invalid')).toBeDefined()
  })

  it('朝向用 data-orientation 表达，默认 vertical', () => {
    expect(mount(UiField).attributes('data-orientation')).toBe('vertical')
    expect(mount(UiField, { props: { orientation: 'horizontal' } }).attributes('data-orientation')).toBe('horizontal')
  })

  it('是 group 语义（读屏能把标签与控件读成一组）', () => {
    expect(mount(UiField).attributes('role')).toBe('group')
  })
})

describe('UiFieldLabel / UiFieldDescription', () => {
  it('label 带 for；required 时渲染星号且对读屏隐藏', () => {
    const w = mount(UiFieldLabel, { props: { forId: 'name', required: true }, slots: { default: '姓名' } })
    expect(w.attributes('for')).toBe('name')
    expect(w.text()).toContain('姓名')
    const star = w.find('[aria-hidden="true"]')
    expect(star.exists()).toBe(true)
    expect(star.text()).toBe('*')
  })

  it('不传 forId 时不渲染 for（组标签没有单一控件）', () => {
    const w = mount(UiFieldLabel, { slots: { default: '通知方式' } })
    expect(w.attributes('for')).toBeUndefined()
    expect(w.find('[aria-hidden="true"]').exists()).toBe(false)
  })

  it('说明行渲染为段落文本', () => {
    const w = mount(UiFieldDescription, { slots: { default: '与证件一致' } })
    expect(w.element.tagName).toBe('P')
  })
})

describe('UiFieldError（错误，支持多条）', () => {
  it('单条错误直出文本', () => {
    const w = mount(UiFieldError, { props: { errors: ['手机号格式不对'] } })
    expect(w.attributes('role')).toBe('alert')
    expect(w.text()).toBe('手机号格式不对')
    expect(w.findAll('li')).toHaveLength(0)
  })

  it('多条错误渲染列表，并去重', () => {
    const w = mount(UiFieldError, { props: { errors: ['必填', '格式不对', '必填', '', null] } })
    const items = w.findAll('li')
    expect(items).toHaveLength(2)
    expect(items.map((i) => i.text())).toEqual(['必填', '格式不对'])
  })

  it('没有错误时不渲染节点（不占一行空白）', () => {
    const w = mount(UiFieldError, { props: { errors: [] } })
    expect(w.find('[role="alert"]').exists()).toBe(false)
    // v-if 关闭时只剩注释占位，没有文本
    expect(w.text()).toBe('')
  })

  it('插槽优先于 errors', () => {
    const w = mount(UiFieldError, { props: { errors: ['不该出现'] }, slots: { default: '<span>自定义</span>' } })
    expect(w.text()).toBe('自定义')
  })
})

describe('UiFieldGroup', () => {
  it('渲染一组字段的容器', () => {
    const w = mount(UiFieldGroup, { slots: { default: '<div>字段</div>' } })
    expect(w.attributes('data-slot')).toBe('field-group')
    expect(w.text()).toContain('字段')
  })
})
