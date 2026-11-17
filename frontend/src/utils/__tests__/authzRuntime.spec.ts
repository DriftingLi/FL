// 运行时能力判定（#1618 段1 / #2600）：静态角色读生成表、动态角色（管理端与讲师）只认运行时集合，未加载即 fail closed。
import { describe, it, expect } from 'vitest'
import { holdsCapability, isDynamicRole } from '@/utils/authzRuntime'

describe('isDynamicRole', () => {
  it('只有生成物声明的动态角色为真（#2600 起 = admin 与 tutor）', () => {
    expect(isDynamicRole('admin')).toBe(true)
    expect(isDynamicRole('tutor')).toBe(true)
    expect(isDynamicRole('hrwai_user')).toBe(false)
    expect(isDynamicRole('recruiter')).toBe(false)
    expect(isDynamicRole('')).toBe(false)
    expect(isDynamicRole(null)).toBe(false)
    expect(isDynamicRole(undefined)).toBe(false)
  })
})

describe('holdsCapability', () => {
  it('静态角色读生成的能力表（运行时空集不影响它）', () => {
    expect(holdsCapability('hrwai_user', [], 'course.learn')).toBe(true)
    expect(holdsCapability('hrwai_user', [], 'forum.moderate')).toBe(false)
    expect(holdsCapability('recruiter', [], 'job.manage')).toBe(true)
  })

  it('动态角色只认运行时能力集', () => {
    expect(holdsCapability('admin', ['forum.moderate'], 'forum.moderate')).toBe(true)
    expect(holdsCapability('admin', ['forum.moderate'], 'audit.read')).toBe(false)
    expect(holdsCapability('tutor', ['question.author'], 'question.author')).toBe(true)
    expect(holdsCapability('tutor', ['question.author'], 'course.author')).toBe(false)
  })

  it('动态角色未加载能力集时一律 false —— 不回落静态表（该表对 admin/tutor 有意为空）', () => {
    expect(holdsCapability('admin', [], 'admin.access')).toBe(false)
    expect(holdsCapability('admin', [], 'forum.moderate')).toBe(false)
    expect(holdsCapability('admin', [], 'audit.read')).toBe(false)
    expect(holdsCapability('tutor', [], 'question.author')).toBe(false)
    expect(holdsCapability('tutor', [], 'tutor.access')).toBe(false)
  })
})
