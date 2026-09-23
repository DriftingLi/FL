/**
 * 论坛**格式轴**的真执行链夹具：`markdown.uts` → `forumBody.uts` → `forumDisplay.uts`
 *
 * 为什么要有这一处（#1273）：`utils/forumDisplay.uts` 从本票起 import 格式轴（列表摘要要先剥成
 * 纯文本，根 ADR-0044），于是**载入它就必须先把上游两层真执行出来**。`utsHarness.loadUts` 是
 * fail-closed 的（缺绑定即抛错），三个在跑 forumDisplay 的套件（`forumBodyBehavior` /
 * `forumImagePickerBehavior` / `markdownToolbarBehavior`）各自抄一份「先 md 再 body 再 display」
 * 的注入表 = 三处需要同步的事实，注入表一变（下次给 forumDisplay 加依赖）就要同时改三处、
 * 漏一处只是多一个红。故收成这一处。
 *
 * **只出模块、不做断言**（口径同 `contractHarness.js`）：判据留在各套件里。
 *
 * ⚠️ 每层都可传**替换后的文件路径**（成对取证把源码改坏后落临时目录再真执行），其余层仍取真源 ——
 * 注入的解析器始终是**真执行**出来的那一份，不是手抄镜像（③ 判据②「它测的是该测的那一支吗」）。
 */
const path = require('path');

const { loadUts } = require('./utsHarness');

const MD_UTS = path.join(__dirname, 'markdown.uts');
const BODY_UTS = path.join(__dirname, 'forumBody.uts');
const DISPLAY_UTS = path.join(__dirname, 'forumDisplay.uts');

/** 解析器与档位常量（真执行的 `utils/markdown.uts`）。`file` 传**变异副本**时用它（成对取证）。 */
function markdownModule(file) {
  return loadUts(file || MD_UTS, {});
}

/** 格式轴（真执行的 `utils/forumBody.uts`，注入上游那一份解析器） */
function forumBodyModule(md, file) {
  const m = md || markdownModule();
  return loadUts(file || BODY_UTS, {
    parseMarkdown: m.parseMarkdown,
    SUBSET_FORUM: m.SUBSET_FORUM,
    SUBSET_CHAPTER: m.SUBSET_CHAPTER,
  });
}

/** 展示层（真执行的 `utils/forumDisplay.uts`，注入上游那一份格式轴） */
function forumDisplayModule(body, file) {
  const b = body || forumBodyModule();
  return loadUts(file || DISPLAY_UTS, {
    forumContentPlainText: b.forumContentPlainText,
    FORMAT_TEXT: b.FORMAT_TEXT,
  });
}

/** 一次拿到三层（各套件按需要的那一层取用；`overrides` 传 `{ md?, body?, display? }` 变异副本路径） */
function forumChain(overrides) {
  const o = overrides || {};
  const md = markdownModule(o.md);
  const body = forumBodyModule(md, o.body);
  return { md, body, display: forumDisplayModule(body, o.display) };
}

module.exports = {
  MD_UTS,
  BODY_UTS,
  DISPLAY_UTS,
  markdownModule,
  forumBodyModule,
  forumDisplayModule,
  forumChain,
};
