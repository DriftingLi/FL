// 表单字段容器的证据面（ADR-0053 §8 票② / spec #1056）。
//
// 四种组合逐一钉住（标签 / 必填 / 无错误 / 有错误），外加两条「纯增量纪律」：
// 不传 required 与 error 时，渲染出的 DOM 与改造前逐字一致（不多节点、不多属性）。
import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import UiFormField from '../UiFormField.vue'

const mountField = (props: Record<string, unknown>, slot = '<input class="form-control" />') =>
  mount(UiFormField, { props: { label: '品牌', ...props }, slots: { default: slot } })

describe('UiFormField', () => {
  it('只给标签：一个 .field 容器 + 一个 .field-label + 插槽内容，别无其他节点', () => {
    const w = mountField({})
    expect(w.find('.field').exists()).toBe(true)
    expect(w.find('.field-label').text()).toBe('品牌')
    expect(w.find('input.form-control').exists()).toBe(true)
    expect(w.find('.field-required').exists()).toBe(false)
    expect(w.find('.field-error').exists()).toBe(false)
    // 纯增量：不传 forId 时不渲染 for（与改造前的组标签一致）
    expect(w.find('.field-label').attributes('for')).toBeUndefined()
  })

  it('forId：渲染 for，指向控件 id（改造前 select 字段的 a11y 契约）', () => {
    const w = mountField({ forId: 'vh-brand' }, '<select id="vh-brand" class="form-control" />')
    expect(w.find('.field-label').attributes('for')).toBe('vh-brand')
  })

  it('required：追加必填标记（默认不渲染，故老调用方零视觉变更）', () => {
    const w = mountField({ required: true })
    expect(w.find('.field-required').text()).toBe('*')
  })

  it('error：渲染错误行且带 role=alert（提示可被读屏播报）', () => {
    const w = mountField({ error: '请选择品牌' })
    const err = w.find('.field-error')
    expect(err.exists()).toBe(true)
    expect(err.text()).toBe('请选择品牌')
    expect(err.attributes('role')).toBe('alert')
  })

  it('error 为空串 / undefined：不渲染错误行', () => {
    expect(mountField({ error: '' }).find('.field-error').exists()).toBe(false)
    expect(mountField({ error: undefined }).find('.field-error').exists()).toBe(false)
  })

  it('插槽内容的事件透传：控件自身的事件照常冒泡到调用方', async () => {
    const w = mount(UiFormField, {
      props: { label: '累计工时' },
      slots: { default: '<input class="form-control" />' }
    })
    const input = w.find('input')
    await input.setValue('123')
    expect((input.element as HTMLInputElement).value).toBe('123')
    await input.trigger('change')
    expect(w.emitted('change')).toBeTruthy()
  })

  it('禁用态标签变灰的选择器能命中：容器 :has(插槽里的 .form-control:disabled)', () => {
    // vitest 不把 SFC 的 <style scoped> 注入文档，故这里断言**选择器的命中形态**——
    // 变灰规则本身写在组件内（`components/ui/UiFormField.vue`），选择器依赖的
    // `.form-control:disabled` 是插槽内容（在父作用域编译）——这条断言把该依赖钉住。
    const disabled = mountField({ forId: 'vh-x' }, '<select id="vh-x" class="form-control" disabled />')
    expect(disabled.find('.field:has(.form-control:disabled)').exists()).toBe(true)

    const enabled = mountField({ forId: 'vh-x' }, '<select id="vh-x" class="form-control" />')
    expect(enabled.find('.field:has(.form-control:disabled)').exists()).toBe(false)
  })
})
