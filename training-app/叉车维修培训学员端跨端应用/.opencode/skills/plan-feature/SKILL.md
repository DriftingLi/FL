---
name: plan-feature
description: >-
  Structured feature planning workflow. Use when the user says "plan feature",
  "做计划", "写 PRD", "planning", or wants to plan a new feature before
  implementation. Covers steps 1-4: free-form plan → PRD → issues → tasks.
---

# Plan Feature

A structured planning workflow that forces thinking before coding. Based on the
principle: "用写作思考，而不是用代码" (think with writing, not code).

## When to Use

- User describes a new feature idea
- User wants to plan before implementing
- User says "plan feature", "做计划", "写 PRD"
- User has a rough idea but needs structure

## Workflow Overview

```
Free-form Plan → PRD → Issues → Tasks
     (1)          (2)     (3)      (4)
```

Each step produces a file. Each file is reviewed by the user before proceeding.

## Step 1: Free-form Plan

**Input**: User's rough description of the problem, ideas, constraints.

**Your job**:
1. Listen to the user's description
2. Ask clarifying questions about unclear points
3. Explore the codebase to understand current state
4. Write a free-form plan document

**Output**: `docs/plans/{feature-name}/plan.md`

**Template**:

```markdown
# {Feature Name} — 自由形式计划

> 创建时间：{date}

## 问题描述

{What problem are we solving? Who is affected?}

## 初步想法

{Rough solution ideas, no structure required}

## 约束条件

- {Technical constraints}
- {Business constraints}
- {Time/resource constraints}

## 不确定的地方

- {What you're unsure about}
- {What needs investigation}

## 相关代码

- {Files/modules that will be affected}
```

**Key rules**:
- This is NOT a deliverable. It's thinking material.
- No required structure — write however helps you think.
- The quality of everything downstream depends on this step.

## Step 2: Generate PRD

**Input**: The free-form plan from Step 1.

**Your job**:
1. Read the plan thoroughly
2. Explore the codebase to understand current state
3. Conduct a structured interview — question EVERY aspect of the plan
4. For each decision, ask:
   - "当用户未认证时，这会如何表现？"
   - "如果此操作部分失败会发生什么？"
   - "你说这会替换现有功能，那依赖当前行为的用户会怎样？"
5. Resolve all dependencies between decisions
6. Generate a structured PRD

**Output**: `docs/plans/{feature-name}/PRD.md`

**Template**:

```markdown
# {Feature Name} — PRD

> 创建时间：{date}
> 状态：Draft | Approved

## 问题陈述

{Clear, concise problem statement}

## 解决方案描述

{High-level solution approach}

## 用户故事

### US-1: {Story Title}

**作为** {role}
**我想要** {action}
**以便** {benefit}

**验收标准**:
- [ ] Given {context}, when {action}, then {outcome}
- [ ] Given {context}, when {action}, then {outcome}

**边缘情况**:
- {What happens when X is null/empty/invalid}
- {What happens when concurrent users do Y}

### US-2: {Story Title}

...

## 实现决策

### 模块设计

{Which modules/services are affected}

### 接口变更

{API contracts, type changes}

### 数据模型

{Database schema changes}

### 模式变更

{Architecture pattern changes if any}

## 范围外

- {What we explicitly NOT doing}
- {Future considerations}
```

**Key rules**:
- The interview process is where bad ideas get discovered.
- AI is not smarter than you — but being forced to answer specific questions
  reveals your vague assumptions.
- User stories must be specific enough to derive acceptance criteria downstream.

## Step 3: Generate Issues

**Input**: The approved PRD.

**Your job**:
1. Read the PRD thoroughly
2. Break it into vertical slices (not horizontal layers)
3. Each slice must be an end-to-end path through all integration layers
4. Classify each issue as AFK (AI can implement) or HITL (needs human decision)
5. Prioritize AFK over HITL to keep momentum

**Output**: `docs/plans/{feature-name}/issues.md`

**Template**:

```markdown
# {Feature Name} — Issues

> 创建时间：{date}

## 问题分解

每个 issue 是一个垂直切片，贯穿所有集成层的端到端路径。

### Issue 1: {Title}

**类型**: AFK | HITL
**依赖**: None | Issue N

**端到端行为**:
{What happens from user action to database and back}

**如何验证**:
{How to confirm this slice is complete}

**验收标准**:
- Given {precondition}, when {action}, then {outcome}
- Given {error case}, when {action}, then {error handling}

**阻塞项**:
- {What needs to be resolved before this can start}

**解决的用户故事**:
- US-{N}: {Story title}

---

### Issue 2: {Title}

...
```

**Key rules**:
- Vertical slices: "只触及数据库或只触及 UI 的切片不是有效切片"
- AFK vs HITL classification keeps work moving without human bottleneck
- Issues are written with real cross-references

## Step 4: Generate Tasks

**Input**: The approved issues.

**Your job**:
1. Read all issues
2. For each issue, break it into concrete, ordered tasks
3. Each task = one focused AI session
4. Follow the order: Pattern → Logic → API → UI → Test
5. Tasks are instructions for the AI that will execute them

**Output**: `docs/plans/{feature-name}/tasks.md`

**Template**:

```markdown
# {Feature Name} — Tasks

> 创建时间：{date}

## Issue 1: {Title}

### Task 1.1: {Title}

**类型**: WRITE | TEST | MIGRATE | CONFIG | REVIEW
**依赖**: None | Task N.M

**范围**:
- 要触及的文件: {file paths}
- 要遵循的模式: {existing patterns to follow}

**任务描述**:
{What to implement, written as instructions to AI}

**完成标准**:
{What the output looks like when done}

---

### Task 1.2: {Title}

...

## Issue 2: {Title}

...
```

**Key rules**:
- "如果一个任务不能在单次会话中完成，那它就太大了"
- Pattern before logic, logic before API, API before UI, tests interspersed
- Task descriptions are "写给将执行它们的 AI 的指令，而不是给人类开发者的笔记"
- No code snippets — "意图，不是实现"

## Before Proceeding

After generating each document, present it to the user and ask:

1. **Plan**: "这个计划是否准确反映了你的想法？有什么遗漏？"
2. **PRD**: "这些用户故事是否完整？验收标准是否清晰？"
3. **Issues**: "粒度是否合适？依赖关系是否正确？"
4. **Tasks**: "任务分解是否合理？是否有遗漏的步骤？"

Wait for user confirmation before proceeding to the next step.
