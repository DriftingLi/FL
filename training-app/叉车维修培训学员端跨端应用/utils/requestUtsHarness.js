/**
 * #1734 两轴拆分后的行为缝装载器：把「纯传输层」与「续期/重试编排」分开装载。
 *
 * 背景：`api/request.uts` 曾同时承担两件事 —— 传输出口（头构造 / URL 解析 / query 序列化 /
 * 信封解析 / 日志脱敏）与续期重试编排（401 三出口、refreshGate 单飞）。#1734 把前者沉成
 * `api/transport.uts`，`request.uts` 只留编排并**从传输层 import** 那一组符号。
 *
 * 对行为缝（`utsHarness`）的影响：`loadUts` 按 import 清单 **fail-closed** 校验绑定齐全，
 * 所以真跑 `request.uts` 的测试必须同时提供传输层符号。手抄那 12 个符号既冗长又与真源漂移
 * （传输层加一个导出就要改 6 处）—— 这里一次性把**真的**传输层模块装载好并按需注入。
 *
 * 用法：
 *   - 要跑 `api/request.uts`：`loadUts(REQUEST_UTS, { ...requestBindings({ uni, getStorage }), ...其他 })`
 *   - 要跑 `api/envelope.uts`：它的 `getPaged` 现在依赖传输层的 `mergeQueryParams`
 *     （`queryMerger()` 取纯函数本体，零外部依赖）
 */
const path = require('path');

const { loadUts } = require('./utsHarness');

/** 纯传输层模块路径（`api/transport.uts`） */
const TRANSPORT_UTS = path.join(__dirname, '..', 'api', 'transport.uts');

/**
 * 装载传输层真实现。
 * @param {Record<string, unknown>} [overrides] 覆盖缺省绑定（`API_BASE_URL` / `STORAGE_KEY_TOKEN` / `getStorage` / `uni`）
 * @returns {Record<string, unknown>} 传输层 `export` 出的名字 → 值
 */
function transport(overrides) {
  const o = overrides || {};
  return loadUts(TRANSPORT_UTS, {
    API_BASE_URL: o.API_BASE_URL == null ? 'http://127.0.0.1:8080' : o.API_BASE_URL,
    STORAGE_KEY_TOKEN: o.STORAGE_KEY_TOKEN == null ? 'auth_token' : o.STORAGE_KEY_TOKEN,
    getStorage: o.getStorage == null ? () => '' : o.getStorage,
    uni: o.uni == null ? { showToast: () => {} } : o.uni,
  });
}

/**
 * `api/request.uts` 的传输层绑定：传输层全部导出 + 两条**转发别名**。
 *
 * `request.uts` 用 `FORBIDDEN_MESSAGE_PREFIX as TRANSPORT_FORBIDDEN_PREFIX` 的形式导入并
 * 再导出同名常量（消费方仍从 `./request` 取），`importedNames` 收集的是**别名**，故这两个键单独给。
 */
function requestBindings(overrides) {
  return transportAliases(transport(overrides));
}

/**
 * 把**任意**一份传输层模块的导出面转成 `request.uts` 可用的绑定（含两条转发别名）。
 *
 * 变异取证（注入坏源码）用得到它：把变异后的 transport 源码装载出来，再经本函数贴别名，
 * 就能在**不改其余接线**的前提下让 request.uts 走坏掉的那份传输层。
 *
 * @param {Record<string, unknown>} t 传输层模块（真源或变异副本）
 */
function transportAliases(t) {
  return Object.assign({}, t, {
    TRANSPORT_FORBIDDEN_PREFIX: t.FORBIDDEN_MESSAGE_PREFIX,
    TRANSPORT_NOT_FOUND_PREFIX: t.NOT_FOUND_MESSAGE_PREFIX,
  });
}

/** 缓存 query 合并纯函数（`mergeQueryParams` 零外部依赖，可跨用例复用） */
let cachedMerger = null;

/** `api/envelope.uts` 的 `mergeQueryParams` 绑定（#1734 问题 3：三处序列化收一后的新依赖） */
function queryMerger() {
  if (cachedMerger == null) {
    cachedMerger = transport().mergeQueryParams;
  }
  return cachedMerger;
}

/**
 * `api/envelope.uts` 的装载绑定：调用方给的 `getMapped` / `toNumber` 原样保留，
 * 再补上它 #1734 起新增的 `mergeQueryParams`（`getPaged` 的 query 合并已收口到传输层）。
 *
 * @param {Record<string, unknown>} [base] 调用方原有绑定
 */
function envelopeBindings(base) {
  return Object.assign({}, base || {}, { mergeQueryParams: queryMerger() });
}

module.exports = { TRANSPORT_UTS, transport, requestBindings, transportAliases, envelopeBindings, queryMerger };