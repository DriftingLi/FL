/**
 * 内容精选 `source` 双形态判别（#1939）—— 行为腿
 *
 * 断言产物（判 true/false 与投影结果），loadUts 真执行 utils/featuredSource.uts。
 * 协议白名单是本票的安全判据：只有 http(s) 才允许变成「查看原文」按钮，
 * 否则一条 `javascript:` 的 source 就成了本 App 的注入入口。
 */
const path = require('path');
const { loadUts } = require('./utsHarness');

const FILE = path.join(__dirname, 'featuredSource.uts');
const mod = loadUts(FILE, {});

describe('内容精选 source 双形态（#1939 定断）', () => {
  it('https / http 完整链接才算外链', () => {
    expect(mod.isExternalSourceUrl('https://www.example.com/a')).toBe(true);
    expect(mod.isExternalSourceUrl('http://example.com')).toBe(true);
    expect(mod.isExternalSourceUrl('HTTPS://EXAMPLE.COM/x')).toBe(true);
  });

  it('来源名称不当外链（存量数据形态）', () => {
    expect(mod.isExternalSourceUrl('和润天下')).toBe(false);
    expect(mod.isExternalSourceUrl('公司官网')).toBe(false);
    expect(mod.isExternalSourceUrl('新华网')).toBe(false);
  });

  it('非 http(s) 协议与相对路径一律不认（安全白名单，不是黑名单）', () => {
    expect(mod.isExternalSourceUrl('javascript:alert(1)')).toBe(false);
    expect(mod.isExternalSourceUrl('data:text/html,<script>')).toBe(false);
    expect(mod.isExternalSourceUrl('ftp://example.com/f')).toBe(false);
    expect(mod.isExternalSourceUrl('/news/2026')).toBe(false);
    expect(mod.isExternalSourceUrl('www.example.com')).toBe(false);
  });

  it('空串与过短串不炸', () => {
    expect(mod.isExternalSourceUrl('')).toBe(false);
    expect(mod.isExternalSourceUrl('http')).toBe(false);
  });

  it('链接形态的展示投影是域名（元信息行不塞整条 URL）', () => {
    expect(mod.sourceDomainOf('https://www.example.com/a/b?x=1')).toBe('www.example.com');
    expect(mod.sourceDomainOf('http://news.gov.cn')).toBe('news.gov.cn');
    expect(mod.sourceDomainOf('http://news.gov.cn/path')).toBe('news.gov.cn');
  });
});
