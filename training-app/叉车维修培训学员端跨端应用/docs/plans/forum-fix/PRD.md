# 论坛 API 参数修复 — PRD

> 创建时间：2026-09-01
> 状态：Draft

## 问题陈述

论坛发帖功能存在 API 参数错误，导致帖子分类不正确。

## 解决方案描述

修复前端 API 调用，使其符合后端规范：
1. 将 `scope` 参数改为 `category`
2. 移除移动端的「招聘」分类
3. 添加渐变背景

## 用户故事

### US-1: 正确的帖子分类

**作为** 用户
**我想要** 发帖时正确选择分类
**以便** 帖子能被正确归类

**验收标准**:
- Given 用户打开提交需求页，when 查看分类选项，then 显示「广场/资源/知识问答」
- Given 用户选择「知识问答」，when 提交帖子，then 帖子 category 为 question

### US-2: 渐变背景

**作为** 用户
**我想要** 页面有渐变背景
**以便** 获得更好的视觉体验

**验收标准**:
- Given 用户打开提交需求页，when 查看页面背景，then 显示渐变背景
- Given 用户打开需求详情页，when 查看页面背景，then 显示渐变背景

## 实现决策

### 模块设计

| 模块 | 文件 | 说明 |
|---|---|---|
| API 层 | `api/forum.uts` | 修复参数名 |
| 提交需求页 | `pages/forum/forum-create.uvue` | 分类选项 + 渐变背景 |
| 需求详情页 | `pages/forum/forum-detail.uvue` | 渐变背景 |

### 数据模型

分类选项变更：
- 移除：`招聘` (recruit)
- 修改值：`广场` → `discussion`，`资源` → `discussion`，`知识问答` → `question`

### API 变更

| 函数 | 当前 | 修复后 |
|---|---|---|
| createForumTopicApi | `scope` 参数 | `category` 参数 |
| updateForumTopicApi | `scope` 参数 | `category` 参数 |

## 范围外

- 招聘功能（仅管理端）
- 其他页面的样式调整
