/**
 * 行为腿「驱动 env」的隔离小件（#2013）。
 *
 * 背景：`dev-finish` 逐套件驱动行为腿时会往子进程注入 `FL_BEHAVIOR_SUITE_OFFSET/TOTAL`
 * （本仓内部契约，消费方 = `scripts/jest-reporter-suite-progress.js`）。于是**同一个变量**有两个
 * 消费者、两种要求：
 *
 *   - `behaviorLegWatchdogBehavior` 的 W6 要**断言**驱动语义（给了 env ⇒ 行里是全局序号）；
 *   - `jestSplitContract` 的 P6 要**隔离**驱动语义（喂合成事件 ⇒ 行里必须是 `total=2`）。
 *
 * 两处此前各写一遍「存原值 → 设/删 → finally 恢复」，逐字同形（#2013 code review 的 Duplicated Code）：
 * env 契约再加第三枚变量时就得改两个文件，正是本仓反复吃亏的「同一件事两份真源」。
 * 故收成一处：**只做 env 的存取，不做断言、不改 reporter**（语义归属仍在 reporter 与本件调用方）。
 *
 * 用法：`withBehaviorSuiteEnv({ offset: 41, total: 75 }, fn)` 设值；
 *      `withBehaviorSuiteEnv({ offset: null, total: null }, fn)` 清空（`null` = 本次调用内删除该键）。
 * 无论 fn 正常返回还是抛出，退出时都按原样恢复（原来是 undefined 的两个键一并删掉）。
 */
'use strict';

/** 驱动 env 的两枚键（reporter 读的那两个，本仓内部契约）。 */
const BEHAVIOR_SUITE_ENV_KEYS = Object.freeze({
  offset: 'FL_BEHAVIOR_SUITE_OFFSET',
  total: 'FL_BEHAVIOR_SUITE_TOTAL',
});

const KEYS = Object.values(BEHAVIOR_SUITE_ENV_KEYS);

function snapshot() {
  return KEYS.map((k) => [k, process.env[k]]);
}

function restore(saved) {
  for (const [k, v] of saved) {
    if (v === undefined) delete process.env[k];
    else process.env[k] = v;
  }
}

/**
 * 在指定的驱动 env 取值下执行 fn，退出时恢复原值。
 * @param {{offset?: string|null, total?: string|null}} values 缺省键不动；显式 null ⇒ 该键在本次调用内被删除
 * @param {() => any} fn
 */
function withBehaviorSuiteEnv(values, fn) {
  const saved = snapshot();
  for (const [slot, key] of Object.entries(BEHAVIOR_SUITE_ENV_KEYS)) {
    if (!Object.prototype.hasOwnProperty.call(values || {}, slot)) continue;
    const v = values[slot];
    if (v === null || v === undefined) delete process.env[key];
    else process.env[key] = String(v);
  }
  try {
    return fn();
  } finally {
    restore(saved);
  }
}

module.exports = { BEHAVIOR_SUITE_ENV_KEYS, withBehaviorSuiteEnv };
