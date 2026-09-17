/**
 * #1111 契约：移动端「显式标记完成」路径必须存在且接线正确。
 *
 * ## 为什么需要这条守护
 *
 * 现象：课程详情页学习进度条**恒 0%**。
 *
 * 根因不是前端显示错，而是**移动端缺了 web 一直有的那条上报路径**：
 * 后端课程进度 = 完成章节数 / 总章节数，而「完成」判据是章节 `progress >= 100`；
 * 章节置 100 有两条路（`backend/internal/service/course_service.go:438`）：
 *   ① `in.Completed` —— **移动端此前从不发送** ⇒ 不可达
 *   ② `StudyDuration >= chapter.Duration` —— 要学满整章才自动完成
 * 两条都不通 ⇒ 完成章节数恒 0 ⇒ 课程进度恒 0。
 *
 * web 端自 ADR-0017 起就走显式路径（`frontend/src/pages/student/ChapterView.vue:374`
 * `updateProgress(courseId, { chapter_id, completed: true })`），移动端没有 ⇒ **两端口径不平等**。
 *
 * ## 断言的是产物，不是源码片段
 *
 * - 载荷形状：从 `updateCourseProgressApi` 体内断言**能产出 `completed` 字段**；
 * - 接线：composable 暴露 `reportCompleted` 且真的以 `completed=true` 调用 API；
 * - 承载面：页面有入口、入口受 `isChapterCompleted` 收口、点击后**重取详情**让后端权威值生效。
 *
 * 刻意**不**钉死具体文案与样式类名（照 ADR-0007 先例：文案/外观可再调整，守护不该拦）。
 */
const fs = require('fs');
const path = require('path');

const ROOT = path.join(__dirname, '..');
/**
 * 读源码并**归一化换行**为 LF。
 *
 * ⚠️ 必须归一：本仓工作树里的 `.uvue` / `.uts` 是 **CRLF**，而 `bodyAfter` 的收尾判据
 * 是 `\n    }\n` —— 不归一就永远匹配不到 `\r\n    }\r\n`，提取器恒返回空串，
 * 于是断言拿到空串、看起来像「接线缺失」的假红（实测踩过：9 条假红）。
 */
const read = (rel) => fs.readFileSync(path.join(ROOT, rel), 'utf8').replace(/\r\n/g, '\n');

const API = 'api/course.uts';
const COMPOSABLE = 'pages/courses/composables/useChapterStudy.uts';
const PAGE = 'pages/courses/chapter-view.uvue';

/**
 * 取某段标记之后的函数体 —— 以**花括号配平**判收尾，不用正则找「独立成行的 }」。
 *
 * 为什么不用更简单的写法（三次实测踩过，别退回去）：
 * - 裸 `indexOf('\n}')`：只对**顶格**函数有效，`<script setup>` 内缩进的页面级函数返回空；
 * - 写死 `\n    }\n`（4 空格）：`.uvue` 是 4 空格缩进但 **`.uts` 是 tab 缩进**，混用 ⇒ 恒不命中；
 * - 宽松版 `\n[ \t]*\}\n`：会被函数体**内部**的 `if { … }` / `try { … }` 闭括号提前截断
 *   （实测把 `updateCourseProgressApi` 截在 `payload['completed'] = true` 处，尾部的
 *   `post(...)` 丢失 ⇒ 假红）。
 *
 * 故唯一稳的判据是数括号。调用方 `read()` 已把 CRLF 归一为 LF。
 */
function bodyAfter(src, marker) {
  const start = src.indexOf(marker);
  if (start < 0) return '';
  const open = src.indexOf('{', start);
  if (open < 0) return '';
  let depth = 0;
  for (let i = open; i < src.length; i++) {
    const ch = src[i];
    if (ch === '{') depth++;
    else if (ch === '}') {
      depth--;
      if (depth === 0) return src.slice(start, i + 1);
    }
  }
  return '';
}

describe('#1111 上报面：updateCourseProgressApi 能发出 completed', () => {
  const src = read(API);

  it('签名接受 completed 参数（且默认 false ⇒ 既有调用方向后兼容）', () => {
    expect(src).toMatch(/export function updateCourseProgressApi\([^)]*completed\s*:\s*boolean\s*=\s*false\s*\)/);
  });

  it('payload 在 completed 为真时写入 completed 字段', () => {
    const body = bodyAfter(src, 'export function updateCourseProgressApi');
    expect(body).not.toBe('');
    // 必须是条件写入（不传时保持原载荷，既有「只报时长」调用方零影响）
    expect(body).toMatch(/if\s*\(\s*completed\s*\)/);
    expect(body).toContain("payload['completed']");
  });

  it('仍保持裸 post 透传（域收紧白名单内唯一 raw 出口，未被误加 DTO）', () => {
    const body = bodyAfter(src, 'export function updateCourseProgressApi');
    expect(body).toContain("post('/course/'");
    expect(body).not.toContain('getMapped');
    expect(body).not.toContain('postMapped');
  });
});

describe('#1111 composable：暴露并实现 reportCompleted', () => {
  const src = read(COMPOSABLE);

  it('UseChapterStudyResult 返回类型含 reportCompleted', () => {
    const typeBody = src.slice(src.indexOf('export type UseChapterStudyResult'), src.indexOf('export function useChapterStudy'));
    expect(typeBody).toContain('reportCompleted');
  });

  it('返回值里 reportCompleted 以箭头包裹（守护规则 T：禁裸函数引用）', () => {
    const ret = bodyAfter(src, 'return {');
    expect(ret).toMatch(/reportCompleted\s*:\s*\(/);
  });

  it('reportCompleted 以 completed=true 调 updateCourseProgressApi', () => {
    const body = bodyAfter(src, 'async function reportCompleted');
    expect(body).not.toBe('');
    expect(body).toMatch(/updateCourseProgressApi\([^)]*true\s*\)/);
  });

  it('未进入章节（目标为 0）时不发请求（沿用既有守卫口径）', () => {
    const body = bodyAfter(src, 'async function reportCompleted');
    expect(body).toMatch(/reportCourseId\s*<=\s*0|reportChapterId\s*<=\s*0/);
  });
});

describe('#1111 承载面：章节课有「标记完成」入口且接线正确', () => {
  const src = read(PAGE);

  it('模板存在受 isChapterCompleted 收口的入口（已完成则收起）', () => {
    expect(src).toMatch(/v-if="!isChapterCompleted"/);
    expect(src).toContain('onMarkCompleted');
  });

  it('isChapterCompleted 以后端 study_status 为唯一判据（不在前端另存完成态）', () => {
    const body = bodyAfter(src, 'const isChapterCompleted');
    expect(body).not.toBe('');
    expect(body).toContain("study_status");
    expect(body).toContain("'completed'");
  });

  it('点击处理器真的发出显式完成上报（走 composable，页面不直接发请求）', () => {
    const body = bodyAfter(src, 'async function onMarkCompleted');
    expect(body).not.toBe('');
    expect(body).toContain('study.reportCompleted()');
    // 页面层零直发请求（T08 收紧口径）
    expect(body).not.toContain('updateCourseProgressApi(');
  });

  it('标记成功后重取章节详情 —— 让 study_status 取后端权威值（web 同款两步语义）', () => {
    const body = bodyAfter(src, 'async function onMarkCompleted');
    expect(body).toContain('await loadDetail()');
  });

  it('标记前先补报剩余时长（时长与完成是两条独立上报，不互相替代）', () => {
    const body = bodyAfter(src, 'async function onMarkCompleted');
    expect(body).toContain('reportIncremental(true)');
    expect(body).toContain('stopStudy()');
  });
});

describe('#1111 兼容性：后端既有自动完成路径不得被本次改动破坏', () => {
  it('本次不触碰后端的时长阈值自动完成逻辑（改口径需另立票并改 Go 测试）', () => {
    const backend = fs.readFileSync(
      path.join(ROOT, '..', '..', 'backend', 'internal', 'service', 'course_service.go'),
      'utf8'
    );
    // 两条置 100 路径必须同时还在：① 学满时长自动 ② 显式 completed
    expect(backend).toMatch(/ch\.StudyDuration\s*>=\s*threshold\s*\|\|\s*in\.Completed/);
    expect(backend).toMatch(/duration\s*>=\s*threshold\s*\|\|\s*in\.Completed/);
  });
});
