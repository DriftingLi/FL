/**
 * #1198 附件简历删除端点 —— **两端同源**契约（源码文本断言）
 *
 * 缺陷本体：移动端 `deleteAttachmentApi()` 打的是 `DELETE /resume/attachment`，
 * 而后端 `resume` 组**从未注册过** `/attachment` 路由 ⇒ 真机必 404；404 时调用方的
 * `.then()` 不执行 ⇒ 本地附件状态清不掉、服务端 PDF 也从未删除。
 *
 * 为什么是源码文本断言：这条不变式的两半分别在两个技术栈里（`.uts` 的调用点 vs Go 的路由注册），
 * 行为测试搭不出这条链路（它只能证明"我 mock 的那个 URL 被我自己的 mock 接住了"）。ADR-0019 的
 * `readText` 归一 EOL，跨平台/CRLF 免疫。
 *
 * 守护（对应票面「判据」）：
 *   A1 移动端实际打的路径 = `/resume/pdf`
 *   A2 后端**确实注册**了 `DELETE /resume/pdf`
 *   A3 错路径 `/resume/attachment` 在两侧都不得回归
 *   A4 函数签名与唯一调用点保持一致（保留 1 个参数，零 `.uvue` 改动）
 */
const path = require('path');

/** 读源码一律经共享读者归一 EOL（ADR-0019）：与检出平台无关，Windows CRLF 也免疫。 */
const { readText } = require('./utsHarness');

const ROOT = path.join(__dirname, '..'); // training-app/<项目名>
const RESUME_UTS = path.join(ROOT, 'api', 'resume.uts');
const ATTACH_UVUE = path.join(ROOT, 'pages', 'resume', 'resume-attach.uvue');
const API_DIR = path.join(ROOT, '..', '..', 'backend', 'internal', 'api');
const JOB_CARD_GO = path.join(API_DIR, 'job_card.go');
const CONTACT_GO = path.join(API_DIR, 'contact.go');

const resumeSrc = readText(RESUME_UTS);
const attachSrc = readText(ATTACH_UVUE);

/** 提取函数体：从函数声明到顶层 `\n}`（先例：utils/concurrent401RefreshContract.test.js） */
function fnBody(src, name) {
  const start = src.indexOf(`function ${name}`);
  if (start === -1) throw new Error(`未找到 ${name}`);
  return src.slice(start, src.indexOf('\n}', start));
}

describe('A1 移动端打的路径 = DELETE /resume/pdf', () => {
  test('deleteAttachmentApi 的函数体里 del() 的目标是 /resume/pdf', () => {
    const body = fnBody(resumeSrc, 'deleteAttachmentApi');
    expect(body).toContain("del('/resume/pdf'");
  });

  test('RequestOptions.url 也同步指向 /resume/pdf（两个字面量不能只改一个）', () => {
    const body = fnBody(resumeSrc, 'deleteAttachmentApi');
    expect(body).toMatch(/url:\s*'\/resume\/pdf'/);
  });
});

describe('A2 后端确实注册了该路由（两端同源）', () => {
  test('backend/internal/api/job_card.go 的 resume 组注册了 DELETE /pdf', () => {
    const src = readText(JOB_CARD_GO);
    expect(src).toMatch(/DELETE\("\/pdf"/);
  });

  test('路由注册点与上传端点同组（防止"只改了移动端"这一类单侧修改）', () => {
    const src = readText(JOB_CARD_GO);
    expect(src).toMatch(/POST\("\/pdf"/);
  });
});

describe('A3 错路径 /resume/attachment 不得回归', () => {
  // ⚠️ 只断言**调用语法**，不断言"文件里出现过这个字符串"：
  //    本文件上方的 JSDoc 正当地提到了历史错路径（"此前打的是 …"），
  //    全文匹配会被自己的注释命中 —— 即 capability-surface.ps1 文件头记的 #1030 教训。
  test('函数体内不存在指向 /resume/attachment 的调用', () => {
    const body = fnBody(resumeSrc, 'deleteAttachmentApi');
    expect(body).not.toContain('/resume/attachment');
  });

  test('全文件不存在 del(\'/resume/attachment\') 这一调用形式', () => {
    expect(resumeSrc).not.toMatch(/del\(\s*'\/resume\/attachment'/);
  });

  test('后端 api 目录里没有 /attachment 的路由注册（按路由语法判，不按字符串）', () => {
    const routeRe = /\.(GET|POST|PUT|DELETE|PATCH)\("\/attachment"/;
    expect(readText(JOB_CARD_GO)).not.toMatch(routeRe);
    expect(readText(CONTACT_GO)).not.toMatch(routeRe);
  });
});

describe('A4 签名与唯一调用点一致（本票零 .uvue 改动的前提）', () => {
  test('函数仍接受 1 个参数（唯一调用点传的是字面量 "pdf"）', () => {
    expect(resumeSrc).toMatch(/function\s+deleteAttachmentApi\s*\(\s*type\s*:\s*string\s*\)/);
  });

  test('唯一调用点仍是 deleteAttachmentApi(\'pdf\')', () => {
    const calls = attachSrc.match(/deleteAttachmentApi\(/g) || [];
    expect(calls).toHaveLength(1);
    expect(attachSrc).toContain("deleteAttachmentApi('pdf')");
  });
});
