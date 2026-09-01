# 论坛 API 参数修复 — Tasks

> 创建时间：2026-09-01

## Issue 1: API 参数修复

### Task 1.1: 修复 createForumTopicApi

**类型**: WRITE
**依赖**: None

**范围**:
- 要触及的文件: `api/forum.uts`
- 要遵循的模式: 参考现有 API 函数

**任务描述**:
修改 `createForumTopicApi` 函数：
- 将参数名 `scope` 改为 `category`
- 默认值从 `'general'` 改为 `'discussion'`
- 更新 payload 中的字段名

**完成标准**:
- 函数签名改为 `createForumTopicApi(title, content, images, category = 'discussion')`
- payload 中使用 `category` 字段

---

### Task 1.2: 修复 updateForumTopicApi

**类型**: WRITE
**依赖**: None

**范围**:
- 要触及的文件: `api/forum.uts`
- 要遵循的模式: 参考现有 API 函数

**任务描述**:
修改 `updateForumTopicApi` 函数：
- 将参数名 `scope` 改为 `category`
- 更新 payload 中的字段名

**完成标准**:
- 函数签名改为 `updateForumTopicApi(topicId, title, content, images, category = '')`
- payload 中使用 `category` 字段

---

## Issue 2: 分类选项修复

### Task 2.1: 修改分类选项

**类型**: WRITE
**依赖**: Task 1.1

**范围**:
- 要触及的文件: `pages/forum/forum-create.uvue`
- 要遵循的模式: 参考现有分类定义

**任务描述**:
修改 `forum-create.uvue` 中的分类选项：
- 移除：`招聘` (recruit)
- 修改值：`广场` → `discussion`，`资源` → `discussion`，`知识问答` → `question`

**完成标准**:
- 分类选项显示「广场/资源/知识问答」
- 选择「知识问答」分类功能正常

---

## Issue 3: 渐变背景

### Task 3.1: 修改 forum-create 背景

**类型**: WRITE
**依赖**: None

**范围**:
- 要触及的文件: `pages/forum/forum-create.uvue`
- 要遵循的模式: 参考 `pages/dashboard/dashboard.uvue` 的渐变背景

**任务描述**:
修改 `forum-create.uvue` 的容器背景：
- 从 `background-color: #f5f5f5`
- 改为 `background: linear-gradient(180deg, #CFE9FB 1%, #D0EBFD 16%, #F5F5F5 100%)`

**完成标准**:
- 页面背景显示渐变效果

---

### Task 3.2: 修改 forum-detail 背景

**类型**: WRITE
**依赖**: None

**范围**:
- 要触及的文件: `pages/forum/forum-detail.uvue`
- 要遵循的模式: 参考 `pages/dashboard/dashboard.uvue` 的渐变背景

**任务描述**:
修改 `forum-detail.uvue` 的容器背景：
- 从 `background-color: #f5f5f5`
- 改为 `background: linear-gradient(180deg, #CFE9FB 1%, #D0EBFD 16%, #F5F5F5 100%)`

**完成标准**:
- 页面背景显示渐变效果
