/**
 * 积分商城「课程兑换」导流的一次性跨页意图 —— **行为级**守护（#2531，真跑 `utils/navigation.uts`）
 *
 * 病根（票面）：课程列表是 tabBar 页，`uni.switchTab` 带不了 query、该页也没有 onLoad ⇒ 商城里的
 * 「去兑换」只能把「进去后请切到积分兑换组」这件事寄存在模块级状态里带过去。这段语义有三个易错点，
 * 都能骗过纯源码断言（`expect(src).toContain('courseRedeemIntent = false')` 在「写了但没清」时照样绿）：
 *
 *   I1 **一次性**：写入只影响**下一次**消费；消费即清空，重复消费返回 false。
 *      （若不清空 ⇒ 用户此后每次切到课程 tab 都被强制跳到积分兑换组，且每次重取一次列表）
 *   I2 **不落盘**：状态住在模块顶层，杀进程即消失。若改用本地存储 ⇒ 冷启动进课程页会莫名单组。
 *   I3 **不串模块实例以外的东西**：写入方（商城页）与消费方（课程页）取的是同一个模块实例，
 *      但**新的模块实例**必须是干净的（这条同时也证明「没走存储」——存储会跨实例存活）。
 *
 * 为什么用行为而非源码断言：#1204 / 外部链接派发两处先例 —— 源码断言回答不了
 * 「我故意弄坏被测物，它会不会红」。这里用 `utils/utsHarness.js` 把**真的** navigation.uts 跑起来，
 * 「必红」一侧由 describe ③ 的**注入变异源码**两条用例给（把清空那行摘掉 / 把初值改成 true）。
 *
 * 边界（照实记）：本文件只跑 `.uts` 逻辑模块；页面侧「CTA 是否真的写意图 + switchTab」「onShow 是否
 * 先消费意图再切组重取」属于 `.uvue`，node 里跑不了 ⇒ 由接线腿的
 * `utils/pointsShopContract.test.js`（AC6）与 `utils/coursesContract.test.js`（AC7）钉住。
 */
const fs = require('fs');
const os = require('os');
const path = require('path');
const { loadUts, readText } = require('./utsHarness');

const ROOT = path.join(__dirname, '..');
const NAVIGATION_UTS = path.join(ROOT, 'utils', 'navigation.uts');

/** 假 `uni` / `getCurrentPages`：一次性意图本身不触任何 uni API，给最小实现即可（goBack 才用） */
const makeBindings = () => ({
  uni: { reLaunch() {}, navigateBack() {} },
  getCurrentPages: () => [],
});

/** 每次都是**全新模块实例**（模块级状态互不串，正是「冷启动」的模型） */
const freshModule = () => loadUts(NAVIGATION_UTS, makeBindings());

describe('课程兑换一次性跨页意图（#2531，真跑 utils/navigation.uts）', () => {
  describe('① 一次性语义', () => {
    it('没人写过 ⇒ 消费返回 false（课程页正常进入不该被切组）', () => {
      const nav = freshModule();
      expect(nav.consumeCourseRedeemIntent()).toBe(false);
    });

    it('写入 → 消费得过 → 再消费得无（消费即清空）', () => {
      const nav = freshModule();
      nav.requestCourseRedeemIntent();
      expect(nav.consumeCourseRedeemIntent()).toBe(true);
      expect(nav.consumeCourseRedeemIntent()).toBe(false);
      expect(nav.consumeCourseRedeemIntent()).toBe(false);
    });

    it('连续写两次仍只给一次（写入不累加、不做队列）', () => {
      const nav = freshModule();
      nav.requestCourseRedeemIntent();
      nav.requestCourseRedeemIntent();
      expect(nav.consumeCourseRedeemIntent()).toBe(true);
      expect(nav.consumeCourseRedeemIntent()).toBe(false);
    });

    it('消费之后再写入，下一次消费重新给 true（可反复使用）', () => {
      const nav = freshModule();
      nav.requestCourseRedeemIntent();
      expect(nav.consumeCourseRedeemIntent()).toBe(true);
      nav.requestCourseRedeemIntent();
      expect(nav.consumeCourseRedeemIntent()).toBe(true);
    });
  });

  describe('② 不落盘 / 不跨实例残留', () => {
    it('写入后换一个模块实例（= 杀进程重启）读不到旧意图', () => {
      const before = freshModule();
      before.requestCourseRedeemIntent();
      const after = freshModule();
      expect(after.consumeCourseRedeemIntent()).toBe(false);
    });

    it('模块自身不碰本地存储（不引 storage、不调 uni.setStorageSync / getStorageSync）', () => {
      const src = readText(NAVIGATION_UTS);
      expect(src).not.toMatch(/setStorageSync|getStorageSync|localStorage|utils\/storage/);
    });
  });

  describe('③ 成对取证：注入变异源码，上面的断言必须变红', () => {
    /** 把真源按 `replacements` 改写成一份**临时副本**并载入；锚点失效即报错（不让变异静默不生效） */
    const loadMutated = (replacements) => {
      let src = readText(NAVIGATION_UTS);
      for (const [from, to] of replacements) {
        expect([from, src.includes(from)]).toEqual([from, true]);
        src = src.split(from).join(to);
      }
      const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'nav-intent-'));
      const file = path.join(dir, 'navigation.uts');
      fs.writeFileSync(file, src, 'utf8');
      return loadUts(file, makeBindings());
    };

    it('把「消费即清空」那行摘掉 ⇒ 重复消费会继续返回 true（现断言必红）', () => {
      const nav = loadMutated([['\tcourseRedeemIntent = false\n', '']]);
      nav.requestCourseRedeemIntent();
      expect(nav.consumeCourseRedeemIntent()).toBe(true);
      // 真源里这里必须是 false；变异副本原样返回 true ⇒ 证明 ① 的第三条断言是活的
      expect(nav.consumeCourseRedeemIntent()).toBe(true);
    });

    it('把初值改成 true ⇒ 没人写过也消费得过（现断言必红）', () => {
      const nav = loadMutated([['let courseRedeemIntent : boolean = false', 'let courseRedeemIntent : boolean = true']]);
      expect(nav.consumeCourseRedeemIntent()).toBe(true);
    });
  });
});
