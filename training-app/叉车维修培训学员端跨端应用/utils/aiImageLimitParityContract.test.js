/**
 * 随问图片**单轮上限的跨栈同源**守护（#2299）
 *
 * 守的是什么：票面验收「单轮图片上限常量与服务端同值」+ 决策 12「客户端先拦，不把拒绝留给服务端」。
 * 后端 `diagnosisMaxImages`（`backend/internal/aiassistant/diagnosis_adapter.go`）决定送上游的
 * **前 3 张**；客户端若不看同一个数，第四张会被静默截掉——学员以为发了、对方只收到三张。
 *
 * 为什么不是「写注释提醒同步」：本仓那条先例（#2298 的超时同源）只是注释，代价是两处各写一份数、
 * 漂移只能靠人盯。**能机检的就别靠注释**：本守护逐字读两侧的字面量比对（先例：
 * `diagnosisComposeParityContract` 比 .uts 与 Web 源码；这里比 .uts 与 Go 源码）。
 *
 * 本文件是**接线守护**（读源码文本，不执行 .uts）；真执行的那一半在
 * `aiImagePickerBehavior.test.js`（行为腿：名额闸门 / 超限文案 / 纯图轮次）。
 */
const path = require('path');
const { readText } = require('./utsHarness');

/** 工作树根 = 移动端项目目录往上三级（`training-app/<项目>` ⇒ 仓根） */
const REPO_ROOT = path.join(__dirname, '..', '..', '..');
const BACKEND_ADAPTER = path.join(REPO_ROOT, 'backend', 'internal', 'aiassistant', 'diagnosis_adapter.go');
const ENV_FILE = path.join(__dirname, '..', 'config', 'env.uts');
const PICKER_FILE = path.join(__dirname, '..', 'composables', 'useAiImagePicker.uts');
const FEATURE_PAGE = path.join(__dirname, '..', 'pages', 'ai-assistant', 'ai-feature.uvue');

/** 取源码里唯一一个数字字面量；取不到 = 锚点漂了，抛错即红（不静默降级） */
function numOf(src, re, what) {
  const m = src.match(re);
  if (!m) throw new Error('找不到 ' + what + '（锚点失效，守护必须同步更新）');
  return Number(m[1]);
}

describe('随问图片单轮上限：客户端随手服务端走（同一处真源）', () => {
  it('AI_MAX_IMAGES == 后端 diagnosisMaxImages（改一边不改另一边必红）', () => {
    const server = numOf(readText(BACKEND_ADAPTER), /diagnosisMaxImages\s*=\s*(\d+)/, 'diagnosisMaxImages');
    const client = numOf(readText(ENV_FILE), /export const AI_MAX_IMAGES\s*=\s*(\d+)/, 'AI_MAX_IMAGES');
    expect(client).toBe(server);
  });

  it('pick 侧不写死数字：只经 AI_MAX_IMAGES + 名额纯函数（换常量即换行为）', () => {
    const picker = readText(PICKER_FILE);
    expect(picker).toMatch(/AI_MAX_IMAGES/);
    expect(picker).toMatch(/remainingSlots\(AI_MAX_IMAGES/);
    expect(picker).toMatch(/capNotice\(AI_MAX_IMAGES\)/);
    // 页面也不得自带第二个上限字面量：托盘与输入框都从同一常量取
    expect(readText(FEATURE_PAGE)).toMatch(/AI_MAX_IMAGES/);
  });

  it('前端先拦：选图前与上传循环里各有一道名额闸门（只留一处 ⇒ 并发多选会超限）', () => {
    const picker = readText(PICKER_FILE);
    const gates = picker.split('remainingSlots(').length - 1;
    // 选图前的「已满」闸门 + count 取剩余额度 + 上传循环里的第二道 —— 三处缺一即红
    expect(gates).toBeGreaterThanOrEqual(3);
  });
});
