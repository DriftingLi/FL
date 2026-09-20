/**
 * #1199 投递态回流 —— 接线契约（源码文本断言）
 *
 * 缺陷本体：投递态只活在页面本地（`job-detail.uvue` 的 `hasApplied = ref<boolean>(false)`），
 * 页面重建即归零 ⇒ 投过的职位按钮复活、再点吃 400（`not_hired` 时是 `ErrApplyCooldown`
 * 「该职位 30 天内暂不能再次投递」）。后端**早已**按学员视角回填 `apply_state` / `cooldown_days`
 * （`job_posting_service.go` 的 `fillApplyStates`，两个学员端点都带），缺的只是客户端消费。
 *
 * 为什么是源码文本断言：状态逻辑在 `.uvue` 的 `<script setup>` 里，现有的 `.uts` 执行缝
 * （`utils/utsHarness.js`）跑不了页面脚本；真机逐页截图（①a）看的是渲染结果，看不到
 * "本地量是否还在"这类接线。两者互补，故本套件钉接线、①a 钉渲染。
 *
 * 守护：
 *   C1 `api/job.uts`：类型声明两字段，且**两处**手动映射（详情 + 列表）都补上（ADR-0003）
 *   C2 `job-detail.uvue`：三态由 `applyState()` 驱动；本地 `hasApplied` 不得回归；
 *      `onApply` 先判态再发请求
 *   C3 `job-list.uvue`：列表项按态收敛，不可投时是**状态标签而非可点伪按钮**
 *   C4 `cooldown_days` 参与运算处必须 `?? 0` 兜底（AGENTS.md 的 Kotlin nullable receiver 坑位）
 */
const path = require('path');

/** 读源码一律经共享读者归一 EOL（ADR-0019）：与检出平台无关，Windows CRLF 也免疫。 */
const { readText } = require('./utsHarness');

const ROOT = path.join(__dirname, '..');
const JOB_UTS = path.join(ROOT, 'api', 'job.uts');
const DETAIL_UVUE = path.join(ROOT, 'pages', 'jobs', 'job-detail.uvue');
const LIST_UVUE = path.join(ROOT, 'pages', 'jobs', 'job-list.uvue');

const jobSrc = readText(JOB_UTS);
const detailSrc = readText(DETAIL_UVUE);
const listSrc = readText(LIST_UVUE);

/** 提两个锚点之间的源码（.uvue 里的函数体是缩进的，见先例 concurrent401RefreshContract.test.js） */
function between(src, from, to) {
  const start = src.indexOf(from);
  if (start === -1) throw new Error(`未找到锚点 ${from}`);
  const end = src.indexOf(to, start + from.length);
  if (end === -1) throw new Error(`未找到结束锚点 ${to}`);
  return src.slice(start, end);
}

/** 模板里的一段（用于断言"按钮是条件渲染的 / 状态标签不可点"） */
function jobActionsBlock() {
  return between(listSrc, '<view class="job-actions">', '<!-- 加载更多 -->');
}

describe('C1 api/job.uts：类型与两处手动映射都要补（ADR-0003）', () => {
  test('JobPosting 声明 apply_state 与 cooldown_days', () => {
    const typeBlock = between(jobSrc, 'export type JobPosting = {', '\n}');
    expect(typeBlock).toMatch(/apply_state\s*\?:\s*string/);
    expect(typeBlock).toMatch(/cooldown_days\s*\?:\s*number/);
  });

  test('两处映射点（详情 / 列表）都写了这两个字段', () => {
    expect((jobSrc.match(/apply_state:\s*\(/g) || []).length).toBeGreaterThanOrEqual(2);
    expect((jobSrc.match(/cooldown_days:\s*/g) || []).length).toBeGreaterThanOrEqual(2);
  });
});

describe('C2 job-detail.uvue：三态由服务端态驱动，本地布尔量不得回归', () => {
  test('本地 hasApplied 布尔量已删除（回归钉死）', () => {
    expect(detailSrc).not.toMatch(/const\s+hasApplied\s*=/);
    expect(detailSrc).not.toMatch(/hasApplied\s*\.value/);
  });

  test('模板三分支：可投 / 已投递 / 冷却中文案', () => {
    expect(detailSrc).toContain("applyState() == 'none'");
    expect(detailSrc).toContain("applyState() == 'applied'");
    expect(detailSrc).toContain('{{ cooldownText() }}');
  });

  test('onApply 先判态、再发请求（否则点了吃 400）', () => {
    const body = between(detailSrc, 'async function onApply', '\n    }');
    const guardAt = body.indexOf("applyState() != 'none'");
    const callAt = body.indexOf('applyJobApi(');
    expect(guardAt).toBeGreaterThanOrEqual(0);
    expect(callAt).toBeGreaterThan(guardAt);
  });
});

describe('C3 job-list.uvue：列表项按态收敛，不可投时不是可点伪按钮', () => {
  test('列表项有状态判定与文案函数', () => {
    expect(listSrc).toMatch(/function\s+applyStateOf\s*\(\s*job\s*:\s*JobPosting\s*\)/);
    expect(listSrc).toMatch(/function\s+applyLabelOf\s*\(\s*job\s*:\s*JobPosting\s*\)/);
  });

  test('「立即投递」按钮是条件渲染的（可投才出现）', () => {
    const block = jobActionsBlock();
    const btnAt = block.indexOf('立即投递');
    const condAt = block.indexOf("v-if=\"applyStateOf(job) == 'none'\"");
    expect(condAt).toBeGreaterThanOrEqual(0);
    expect(btnAt).toBeGreaterThan(condAt);
  });

  test('不可投时渲染的是状态标签，且**不带 @click**（伪按钮回归钉死）', () => {
    const block = jobActionsBlock();
    const at = block.indexOf('job-action-disabled');
    expect(at).toBeGreaterThanOrEqual(0);
    const element = block.slice(block.lastIndexOf('<view', at), block.indexOf('</view>', at));
    expect(element).toContain('applyLabelOf(job)');
    expect(element).not.toContain('@click');
  });
});

describe('C4 可空字段参与运算必须 ?? 兜底（Kotlin nullable receiver）', () => {
  test('详情页与列表页的 cooldown_days 都经 ?? 0', () => {
    expect(detailSrc).toMatch(/cooldown_days\s*\?\?\s*0/);
    expect(listSrc).toMatch(/cooldown_days\s*\?\?\s*0/);
  });
});
