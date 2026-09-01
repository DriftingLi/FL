# 功能规划文档

每个功能一个子目录，包含完整的规划文件链。

## 文件结构

```
docs/plans/{feature-name}/
├── plan.md          # 自由形式计划（步骤 1）
├── PRD.md           # 结构化 PRD（步骤 2）
├── issues.md        # 垂直切片 issues（步骤 3）
└── tasks.md         # 任务分解（步骤 4）
```

## 命名约定

- 目录名：kebab-case（如 `job-report`、`ui-spec`）
- 文件名：固定为 `plan.md` / `PRD.md` / `issues.md` / `tasks.md`

## 工作流

1. 使用 `plan-feature` 技能生成文档
2. 使用 `execute-task` 技能执行任务
3. 使用 `review-code` 技能审查代码

## 现有功能

| 功能 | 目录 | 状态 |
|---|---|---|
| 职位举报 | `job-report/` | 规划中 |
