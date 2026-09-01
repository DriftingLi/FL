# {功能名称} — Tasks

> 创建时间：{date}

## Issue 1: {标题}

### Task 1.1: {标题}

**类型**: WRITE | TEST | MIGRATE | CONFIG | REVIEW
**依赖**: None | Task N.M

**范围**:
- 要触及的文件: {file paths}
- 要遵循的模式: {existing patterns to follow}

**任务描述**:
{给 AI 的指令，写清楚要做什么}

**完成标准**:
{完成时的样子，用于验证}

---

### Task 1.2: {标题}

**类型**: WRITE | TEST | MIGRATE | CONFIG | REVIEW
**依赖**: None | Task N.M

**范围**:
- 要触及的文件: {file paths}
- 要遵循的模式: {existing patterns to follow}

**任务描述**:
{给 AI 的指令}

**完成标准**:
{完成时的样子}

---

## Issue 2: {标题}

### Task 2.1: {标题}

**类型**: WRITE | TEST | MIGRATE | CONFIG | REVIEW
**依赖**: None | Task N.M

**范围**:
- 要触及的文件: {file paths}
- 要遵循的模式: {existing patterns to follow}

**任务描述**:
{给 AI 的指令}

**完成标准**:
{完成时的样子}

---

## 任务类型说明

| 类型 | 说明 |
|---|---|
| WRITE | 编写新代码 |
| TEST | 编写测试 |
| MIGRATE | 数据库迁移 |
| CONFIG | 配置变更 |
| REVIEW | 需要人工审查的决策点 |

## 任务分解原则

1. **单任务单会话**：如果一个任务不能在单次会话中完成，那它就太大了
2. **模式先行**：先写模式/类型，再写逻辑，再写 API，再写 UI
3. **测试穿插**：测试不是最后批量处理，而是穿插在任务中
4. **意图而非实现**：任务描述是给 AI 的指令，不是给人类开发者的笔记
