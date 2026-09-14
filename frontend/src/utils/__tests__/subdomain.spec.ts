import { describe, it, expect, vi, afterEach } from 'vitest'
import { getSubdomain, getRoleForSubdomain, getDefaultWorkspaceBySubdomain, buildSubdomainUrl, isSameSiteUrl } from '../subdomain'

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

// 本站地址判定（外链中转口径）：判据是「会不会把读者带离本站」，不是「href 以 / 开头」。
// 后者是本仓曾出现的缺陷 —— 本站绝对地址被当成站外，点站内链接也弹「即将离开本站」。
describe('isSameSiteUrl', () => {
  it('本站域名族的绝对地址都算站内（含其它子域名与根域名）', () => {
    mockHostname('training.example.com')
    expect(isSameSiteUrl('https://training.example.com/forum/1')).toBe(true)
    expect(isSameSiteUrl('https://www.example.com/news')).toBe(true) // 门户站（独立 Nuxt 仓库，但仍是本站）
    expect(isSameSiteUrl('https://example.com/x')).toBe(true) // 根域名（nginx 301 到 www）
    expect(isSameSiteUrl('https://manage.example.com/admin/x')).toBe(true)
    expect(isSameSiteUrl('http://training.example.com/x')).toBe(true) // 协议不同也仍是本站
  })

  it('相对地址 / 锚点 / 协议相对地址都算站内（同源，不会把读者带离本站）', () => {
    mockHostname('training.example.com')
    expect(isSameSiteUrl('/training/courses')).toBe(true)
    expect(isSameSiteUrl('#reply-3')).toBe(true)
    expect(isSameSiteUrl('foo/bar')).toBe(true)
    expect(isSameSiteUrl('?page=2')).toBe(true)
    expect(isSameSiteUrl('//training.example.com/x')).toBe(true)
  })

  it('外站一律不算站内（含后缀相似的仿冒域名）', () => {
    mockHostname('training.example.com')
    expect(isSameSiteUrl('https://other.example.org/x')).toBe(false)
    expect(isSameSiteUrl('https://example.com.evil.net/x')).toBe(false) // 后缀相似但不是本站域名族
    expect(isSameSiteUrl('https://notexample.com/x')).toBe(false)
    expect(isSameSiteUrl('https://training.example.com.evil.net/x')).toBe(false)
  })

  it('无主机名的协议不算站内（mailto: / tel: / 伪协议交给调用方另行分类）', () => {
    mockHostname('training.example.com')
    expect(isSameSiteUrl('mailto:a@b.com')).toBe(false)
    expect(isSameSiteUrl('tel:123456')).toBe(false)
    expect(isSameSiteUrl('javascript:alert(1)')).toBe(false)
    expect(isSameSiteUrl('')).toBe(false)
    expect(isSameSiteUrl('   ')).toBe(false)
  })

  it('IP 直连模式：同一 IP 与其相对地址算站内，别的地址仍算站外', () => {
    mockHostname('192.168.1.10')
    expect(isSameSiteUrl('http://192.168.1.10/training/forum/1')).toBe(true)
    expect(isSameSiteUrl('/training/courses')).toBe(true)
    expect(isSameSiteUrl('https://example.com/x')).toBe(false)
  })

  it('开发环境 *.localhost 视为本站（与 getRootDomain 的口径一致）', () => {
    mockHostname('training.localhost')
    expect(isSameSiteUrl('http://localhost:5173/training/courses')).toBe(true)
    expect(isSameSiteUrl('https://example.com/x')).toBe(false)
  })
})
