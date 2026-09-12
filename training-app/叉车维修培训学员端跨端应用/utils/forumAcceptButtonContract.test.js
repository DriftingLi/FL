/**
 * 采纳按钮可见性契约测试（2026-09-11）
 *
 * 现象：知识问答帖、本人是楼主、帖内有他人回复、未采纳 —— 安卓真机整块不渲染
 * （采纳按钮与「已采纳」徽章都无），**同一账号同一数据在微信小程序正常**。
 *
 * 诊断排除项（均已核对，非本因）：
 * - 后端 DTO 与前端提取器对齐：author.user_id / username / avatar_url；is_accepted 逐回复计算正确
 * - ForumReplyDisplay 映射逐字正确（authorId: r.author_id、isAccepted: r.is_accepted）
 * - category 字段在 ForumTopicDetail 与后端 DTO 中均存在（types 拆分后未丢）
 * - 组件确实被渲染：forum-detail 使用 ForumReplyList，components 下仅此一个回复列表组件
 * - 工作区与 origin/master 完全一致（非版本错位）；本地无未推送提交（「忘记推送」已排除）
 *
 * 判别依据：同为原始 boolean 的 isAccepted 若走到 v-else-if 会渲染出「已采纳」徽章；
 * 徽章与按钮同时消失 ⇒ 失败点在 `topicCategory === 'question' && isTopicOwner` 这一组
 * 原生端模板求值上（多条件链在 Kotlin 侧不稳），而非 isAccepted 那一环。
 *
 * 修法：把判断从模板搬回壳层，扁平下发 canAcceptReply 布尔位——与本文件既有的
 * canDelete / liked 权限位同一先例（组件模板零判断逻辑）。
 *
 * 钉住的契约：
 * 1) 判定在壳层计算，含四个必要条件；模板只读一个布尔位、零条件求值
 * 2) 死代码已清：组件的 topicCategory / currentUserId props 与父组件 topicCategory computed
 * 3) 取消采纳路径未被削弱（楼主 + 已采纳）
 * 4) 触发的仍是既有 acceptReply 事件（onAccept(item.id) 不失效）
 */
const fs = require('fs');
const path = require('path');

const ROOT = path.join(__dirname, '..');
const read = (rel) => fs.readFileSync(path.join(ROOT, rel), 'utf8');

const DETAIL = 'pages/forum/forum-detail.uvue';
const LIST = 'pages/forum/components/forum-reply-list.uvue';
const DISPLAY = 'utils/forumDetailDisplay.uts';

describe('采纳按钮：判定由壳层下发（安卓真机可见性修复）', () => {
  const detail = read(DETAIL);
  const list = read(LIST);

  it('展示模型含 canAcceptReply 布尔位', () => {
    const model = read(DISPLAY);
    expect(model).toMatch(/canAcceptReply : boolean/);
  });

  it('壳层 canAcceptReply 覆盖五个必要条件（含 is_experience 守卫）', () => {
    const start = detail.indexOf('function canAcceptReply');
    expect(start).toBeGreaterThan(-1);
    const body = detail.slice(start, start + 500);
    expect(body).toContain('if (!isTopicOwner.value) return false');
    expect(body).toMatch(/if \(t == null \|\| t\.category != 'question'\) return false/);
    // #836：经验帖不可被采纳（后端守卫双向 + 库层 CHECK，前端先隐藏必然失败的按钮）
    expect(body).toContain('if (t.is_experience) return false');
    expect(body).toContain('if (r.is_accepted) return false');
    expect(body).toContain('if (uid <= 0) return false');
    expect(body).toContain('return r.author_id != uid');
  });

  it('经验帖不渲染采纳按钮（#836：is_experience 守卫在 category 判定之后、其余判定之前）', () => {
    const start = detail.indexOf('function canAcceptReply');
    const body = detail.slice(start, start + 500);
    const categoryGuard = body.indexOf("t.category != 'question'");
    const experienceGuard = body.indexOf('t.is_experience');
    const acceptedGuard = body.indexOf('r.is_accepted');
    expect(categoryGuard).toBeGreaterThan(-1);
    expect(experienceGuard).toBeGreaterThan(categoryGuard);
    expect(acceptedGuard).toBeGreaterThan(experienceGuard);
  });

  it('函数定义在 replyItems 之前（UTS 无函数提升，调早于定义即 error18）', () => {
    const fnAt = detail.indexOf('function canAcceptReply');
    const useAt = detail.indexOf('const replyItems = computed');
    expect(fnAt).toBeGreaterThan(-1);
    expect(useAt).toBeGreaterThan(-1);
    expect(fnAt).toBeLessThan(useAt);
  });

  it('展示模型构造处赋值该位', () => {
    expect(detail).toMatch(/canAcceptReply: canAcceptReply\(r\),/);
  });

  it('模板只读布尔位，不再做多条件求值', () => {
    expect(list).toMatch(/v-if="item\.canAcceptReply"/);
    expect(list.includes("topicCategory === 'question'")).toBe(false);
    expect(list.includes('item.authorId !== currentUserId')).toBe(false);
  });
});

describe('采纳按钮：死代码清理（项目规范：删判断同步删声明）', () => {
  const detail = read(DETAIL);
  const list = read(LIST);

  it('组件不再声明已无消费方的 props', () => {
    const propsAt = list.indexOf('defineProps<');
    const propsBlock = list.slice(propsAt, list.indexOf('}>', propsAt));
    expect(propsBlock.includes('topicCategory')).toBe(false);
    expect(propsBlock.includes('currentUserId')).toBe(false);
  });

  it('父组件不再传入这两个绑定，也不再保留 topicCategory computed', () => {
    expect(detail.includes(':topic-category=')).toBe(false);
    expect(detail.includes(':current-user-id=')).toBe(false);
    expect(detail.includes('const topicCategory')).toBe(false);
  });

  it('isTopicOwner 保留（取消采纳仍需要）且仍被传入', () => {
    expect(list).toMatch(/isTopicOwner\? : boolean/);
    expect(detail).toContain(':is-topic-owner="isTopicOwner"');
  });
});

describe('采纳按钮：链路未被削弱（回归锁）', () => {
  const list = read(LIST);

  it('取消采纳路径仍在（楼主 + 已采纳）', () => {
    expect(list).toMatch(/v-else-if="isTopicOwner && item\.isAccepted"/);
    expect(list).toContain('取消采纳');
  });

  it('已采纳徽章仍在（独立于按钮判定，是本次诊断的判别依据）', () => {
    expect(list).toMatch(/v-if="item\.isAccepted" class="reply-accepted-badge"/);
    expect(list).toContain('已采纳');
  });

  it('点击仍触发既有 acceptReply 事件并携带回复 id', () => {
    expect(list).toMatch(/@click="onAccept\(item\.id\)"/);
    expect(list).toMatch(/emit\('acceptReply', id\)/);
  });

  it('父组件接线未变（onAcceptReplyById / onCancelAcceptById）', () => {
    const detail = read(DETAIL);
    expect(detail).toContain('@accept-reply="onAcceptReplyById"');
    expect(detail).toContain('@cancel-accept="onCancelAcceptById"');
  });
});