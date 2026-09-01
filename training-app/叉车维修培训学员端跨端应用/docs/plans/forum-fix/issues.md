# 论坛 API 参数修复 — Issues

> 创建时间：2026-09-01

## 问题分解

每个 issue 是一个垂直切片，贯穿所有集成层的端到端路径。

---

### Issue 1: API 参数修复

**类型**: AFK
**依赖**: None

**端到端行为**:
修复 `api/forum.uts` 中的 `createForumTopicApi` 和 `updateForumTopicApi`，将 `scope` 参数改为 `category`。验证 API 调用正确。

**如何验证**:
- 调用 `createForumTopicApi` 时传递 `category` 参数
- 后端正确接收并处理 `category` 参数

**验收标准**:
- Given 调用 createForumTopicApi，when 传递 category='question'，then 帖子被正确分类为 question

**阻塞项**: 无

**解决的用户故事**: US-1

---

### Issue 2: 分类选项修复

**类型**: AFK
**依赖**: Issue 1

**端到端行为**:
修改 `forum-create.uvue` 的分类选项，移除「招聘」，将值改为 `discussion`/`question`。验证分类选择功能正常。

**如何验证**:
- 打开提交需求页，分类选项显示「广场/资源/知识问答」
- 选择「知识问答」分类，提交帖子成功

**验收标准**:
- Given 用户打开提交需求页，when 查看分类选项，then 显示「广场/资源/知识问答」
- Given 用户选择「知识问答」分类，when 提交帖子，then 帖子 category 为 question

**阻塞项**: Issue 1

**解决的用户故事**: US-1

---

### Issue 3: 渐变背景

**类型**: AFK
**依赖**: None

**端到端行为**:
为 `forum-create.uvue` 和 `forum-detail.uvue` 添加渐变背景。

**如何验证**:
- 打开提交需求页，背景显示渐变
- 打开需求详情页，背景显示渐变

**验收标准**:
- Given 用户打开提交需求页，when 查看页面背景，then 显示渐变背景
- Given 用户打开需求详情页，when 查看页面背景，then 显示渐变背景

**阻塞项**: 无

**解决的用户故事**: US-2

---

## AFK vs HITL 分类

| 类型 | 说明 |
|---|---|
| AFK | 所有 Issue 都是 AFK，AI 可独立完成 |
