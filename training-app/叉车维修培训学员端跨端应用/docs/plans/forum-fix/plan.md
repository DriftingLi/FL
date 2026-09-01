# 论坛 API 参数修复 — 自由形式计划

> 创建时间：2026-09-01

## 问题描述

论坛发帖功能存在 API 参数错误：
1. 前端使用 `scope` 参数，但后端期望 `category` 参数
2. 分类选项包含「招聘」，但招聘功能仅在管理端
3. 渐变背景未应用

## 初步想法

1. 修改 `api/forum.uts` 中的 `createForumTopicApi` 和 `updateForumTopicApi`，将 `scope` 改为 `category`
2. 修改 `forum-create.uvue` 的分类选项，移除「招聘」，将值改为 `discussion`/`question`
3. 为 `forum-create.uvue` 和 `forum-detail.uvue` 添加渐变背景

## 约束条件

- 遵循后端 API 规范（category 参数）
- 保持现有功能不变
- 遵循项目的 UI 规范

## 不确定的地方

- 资源分类的 category 值是什么？（可能是 `discussion`）
- 渐变背景是否需要应用到所有子元素？

## 相关代码

- `api/forum.uts` — API 封装
- `pages/forum/forum-create.uvue` — 提交需求页
- `pages/forum/forum-detail.uvue` — 需求详情页
- 后端 API 文档：GET /forum/topics, POST /forum/topics
