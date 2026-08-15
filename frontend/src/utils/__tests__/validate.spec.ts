// 格式校验单一事实源（validate.ts）单测：锁定邮箱域名含点契约（与后端 IsValidEmail 对齐）。
import { describe, it, expect } from 'vitest'
import type { FormItemRule } from 'element-plus'
import { isValidEmail, isValidPhone, isValidAccount, confirmPasswordRule } from '../validate'

// 执行一个 FormItemRule 的 validator，返回其回调抛出的 Error（无错误时为 undefined）。
// 走公共接口：直接调用 rule.validator；formRef 由 confirmPasswordRule 闭包捕获，此处仅传 value。
function runValidator(rule: FormItemRule, value: string): Error | undefined {
  let err: Error | undefined
  const validator = rule.validator as (r: unknown, v: unknown, cb: (e?: Error | boolean) => void) => void
  validator(rule, value, (e?: Error | boolean) => {
    if (e instanceof Error) err = e
  })
  return err
}

describe('isValidEmail', () => {
  it('域名含点时返回 true（a@b.c）', () => {
    expect(isValidEmail('a@b.c')).toBe(true)
  })

  it('拒绝无点域（a@b），与后端 IsValidEmail 的域名含点语义对齐', () => {
    expect(isValidEmail('a@b')).toBe(false)
  })

  it('常见合法邮箱返回 true', () => {
    expect(isValidEmail('user@example.com')).toBe(true)
    expect(isValidEmail('USER@Example.COM')).toBe(true)
  })

  it('非法邮箱返回 false', () => {
    expect(isValidEmail('')).toBe(false)
    expect(isValidEmail('plain')).toBe(false)
    expect(isValidEmail('a b@c.d')).toBe(false)
    expect(isValidEmail('@example.com')).toBe(false)
    expect(isValidEmail('user@.com')).toBe(false)
  })
})

describe('isValidPhone', () => {
  it('11 位 1[3-9] 开头为合法手机号', () => {
    expect(isValidPhone('13800138000')).toBe(true)
    expect(isValidPhone('12800138000')).toBe(false)
    expect(isValidPhone('1380013800')).toBe(false)
  })
})

describe('isValidAccount', () => {
  it('与后端 IsValidAccount 对齐：4-20 位字母/数字/下划线', () => {
    expect(isValidAccount('abc1')).toBe(true)
    expect(isValidAccount('a_bc')).toBe(true)
    expect(isValidAccount('abc')).toBe(false)
    expect(isValidAccount('a'.repeat(21))).toBe(false)
    expect(isValidAccount('a-bc')).toBe(false)
  })
})

describe('confirmPasswordRule', () => {
  const formRef = { password: '123456' }

  it('空值：默认文案「请再次输入密码」', () => {
    const rule = confirmPasswordRule(formRef, 'password')
    expect(runValidator(rule, '')?.message).toBe('请再次输入密码')
  })

  it('空值：可选 message 参数化（Forgot「请再次输入新密码」）', () => {
    const rule = confirmPasswordRule(formRef, 'password', '请再次输入新密码')
    expect(runValidator(rule, '')?.message).toBe('请再次输入新密码')
  })

  it('不一致：报告「两次输入密码不一致」', () => {
    const rule = confirmPasswordRule(formRef, 'password')
    expect(runValidator(rule, '654321')?.message).toBe('两次输入密码不一致')
  })

  it('一致：无错误', () => {
    const rule = confirmPasswordRule(formRef, 'password')
    expect(runValidator(rule, '123456')).toBeUndefined()
  })

  it('支持自定义 fieldName 字段比较', () => {
    const form = { newPassword: 'abc123' }
    const rule = confirmPasswordRule(form, 'newPassword')
    expect(runValidator(rule, 'abc123')).toBeUndefined()
    expect(runValidator(rule, 'wrong1')?.message).toBe('两次输入密码不一致')
  })
})