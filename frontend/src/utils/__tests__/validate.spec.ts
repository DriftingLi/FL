// 格式校验单一事实源（validate.ts）单测：锁定邮箱域名含点契约（与后端 IsValidEmail 对齐）。
import { describe, it, expect } from 'vitest'
import { isValidEmail, isValidPhone, isValidAccount } from '../validate'

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
