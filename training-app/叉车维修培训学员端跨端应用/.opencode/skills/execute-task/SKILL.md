---
name: execute-task
description: >-
  Execute a single task from a feature plan. Use when the user says "execute
  task", "执行任务", or wants to implement a specific task from the tasks.md
  file. Each task = one focused AI session with clean context.
---

# Execute Task

Implement a single task from a feature plan. Each task is a self-contained
prompt designed for one focused AI session.

## When to Use

- User says "execute task", "执行任务"
- User wants to implement a specific task
- User provides a task number or description

## Prerequisites

- `docs/plans/{feature-name}/tasks.md` must exist
- `docs/plans/{feature-name}/issues.md` must exist (for context)
- `docs/plans/{feature-name}/PRD.md` must exist (for acceptance criteria)

## Workflow

### 1. Load Context

Read these files to understand the full context:

```
docs/plans/{feature-name}/PRD.md      # What we're building and why
docs/plans/{feature-name}/issues.md   # Which slice this task belongs to
docs/plans/{feature-name}/tasks.md    # The specific task to execute
```

### 2. Understand the Task

From the task file, extract:
- **Scope**: Which files to touch
- **Patterns**: Which existing patterns to follow
- **Dependencies**: What must be done first
- **Completion criteria**: What "done" looks like

### 3. Explore the Codebase

Before writing any code:
1. Read the files mentioned in the task
2. Understand the existing patterns
3. Check for similar implementations to reference
4. Identify any gaps between the task description and reality

### 4. Implement

Follow the task description exactly:
- Write code in the specified files
- Follow the specified patterns
- Handle error cases as defined
- Add tests if the task type is TEST

### 5. Verify

Check against the completion criteria:
- [ ] All specified files are created/modified
- [ ] Code follows existing patterns
- [ ] Error cases are handled
- [ ] No regressions in existing functionality

### 6. Report

Output a summary:

```markdown
## Task {N.M}: {Title} — 完成

### 变更文件
- `path/to/file.ts` — {what changed}
- `path/to/new-file.ts` — {what was created}

### 实现要点
- {Key decisions made}
- {Patterns followed}

### 注意事项
- {Anything that deviated from the plan}
- {Known limitations}

### 验证结果
- [x] Completion criteria met
- [ ] Needs review (if HITL task)
```

## Key Rules

1. **One task per session**: "使用良好范围的任务从干净开始，始终比继续长时间会话产生更好的输出"
2. **Clean context**: Start fresh, don't accumulate context from previous tasks
3. **Follow the plan**: Implement what the task describes, not what you think is better
4. **Report deviations**: If reality differs from the plan, document it

## If Blocked

If the task cannot be completed as described:
1. Document the blocker
2. Suggest a modification to the task
3. Wait for user decision before proceeding
