import { describe, it, expect, vi, afterEach } from 'vitest'
import { getSubdomain, getRoleForSubdomain, getDefaultWorkspaceBySubdomain, buildSubdomainUrl } from '../subdomain'

function mockHostname(host: string) {
  Object.defineProperty(window, 'location', {
    value: new URL(`https://${host}/`),
    writable: true
  })
}

afterEach(() => {
  vi.restoreAllMocks()
})

describe('getSubdomain', () => {
  it('recruit. 前缀解析为 recruit', () => {
    mockHostname('recruit.example.com')
    expect(getSubdomain()).toBe('recruit')
  })
  it('mentor. 解析为 tutor', () => {
    mockHostname('mentor.example.com')
    expect(getSubdomain()).toBe('tutor')
  })
  it('manage. 解析为 admin', () => {
    mockHostname('manage.example.com')
    expect(getSubdomain()).toBe('admin')
  })
})

describe('getRoleForSubdomain', () => {
  it('recruit 子域返回 recruiter', () => {
    mockHostname('recruit.example.com')
    expect(getRoleForSubdomain()).toBe('recruiter')
  })
  it('training 子域返回 hrwai_user', () => {
    mockHostname('training.example.com')
    expect(getRoleForSubdomain()).toBe('hrwai_user')
  })
})

describe('getDefaultWorkspaceBySubdomain', () => {
  it('recruit 子域默认工作区为 /recruit', () => {
    mockHostname('recruit.example.com')
    expect(getDefaultWorkspaceBySubdomain()).toBe('/recruit')
  })
  it('valuation 子域默认工作区为 /valuation', () => {
    mockHostname('valuation.example.com')
    expect(getDefaultWorkspaceBySubdomain()).toBe('/valuation')
  })
})

describe('buildSubdomainUrl', () => {
  it('recruit 目标构建正确 URL', () => {
    mockHostname('training.example.com')
    const url = buildSubdomainUrl('recruit', '/recruit/resumes')
    expect(url).toContain('recruit.')
    expect(url).toContain('/recruit/resumes')
  })
})
