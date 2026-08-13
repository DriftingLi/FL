// displayNameOf：显示名派生唯一实现的接口测试。
import { describe, it, expect } from 'vitest'
import { displayNameOf } from '@/types/user'

describe('displayNameOf（用户资料 module 派生字段）', () => {
  it('hrwai 用户：昵称 username 优先，退回登录账号 account', () => {
    expect(displayNameOf({ role: 'hrwai_user', username: 'alice', account: 'acct_alice' })).toBe('alice')
    expect(displayNameOf({ role: 'hrwai_user', account: 'acct_alice' })).toBe('acct_alice')
  })

  it('讲师/管理员：显示名 name 优先，退回 username/account', () => {
    expect(displayNameOf({ role: 'tutor', name: '导师', username: 'tutor', account: 'tutor' })).toBe('导师')
    expect(displayNameOf({ role: 'admin', name: '系统管理员', username: 'admin', account: 'admin' })).toBe('系统管理员')
    expect(displayNameOf({ role: 'tutor', username: 'tutor', account: 'tutor' })).toBe('tutor')
  })

  it('空用户返回空串（页面各自给角色默认名）', () => {
    expect(displayNameOf(null)).toBe('')
    expect(displayNameOf(undefined)).toBe('')
    expect(displayNameOf({})).toBe('')
  })
})
