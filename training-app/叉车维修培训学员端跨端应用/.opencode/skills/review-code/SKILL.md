---
name: review-code
description: >-
  Structured code review with six review rounds. Use when the user says
  "review code", "代码审查", or wants to review a PR or feature implementation.
  Covers logic errors, operation ordering, bad practices, security, magic
  strings, and pattern improvements.
---

# Review Code

A structured six-round code review process. Reviews run on files or diffs,
not entire features — scope is intentionally narrow.

## When to Use

- User says "review code", "代码审查"
- User wants to review a PR or feature
- Before merging any changes
- After implementing a feature

## Review Rounds

### Round 1: Logic Errors

Check for:
- Off-by-one errors
- Null/undefined handling
- Type mismatches
- Incorrect conditions
- Missing edge cases
- Race conditions

**Focus**: Does the code do what it claims to do?

### Round 2: Operation Ordering

Check for:
- Notifications sent before transaction commits
- Audit logs written after state changes
- Validation happening after mutations
- Cleanup happening before error handling

**Focus**: "模型倾向于产生做正确事情但有时顺序错误的代码"

This is especially important in AI-generated code. The model often:
- Sends notifications before committing transactions
- Writes audit logs after the operation that should be logged
- Validates inputs after changing state

### Round 3: Bad Practices

Check for:
- God functions (too many responsibilities)
- Deep nesting
- Magic numbers/strings
- Duplicated code
- Missing error handling
- Overly complex conditionals
- Unnecessary mutations

**Focus**: Is the code maintainable?

### Round 4: Security

Check for:
- SQL injection
- XSS vulnerabilities
- Hardcoded secrets
- Missing authentication/authorization
- Insecure dependencies
- Data exposure
- Path traversal

**Focus**: Can this be exploited?

### Round 5: Magic Strings and Values

Check for:
- Hardcoded URLs
- Inline color codes
- Unexplained constants
- Missing configuration
- Environment-specific values

**Focus**: Are these values documented and configurable?

### Round 6: Pattern Improvements

Check for:
- Inconsistent patterns with existing code
- Missed opportunities to use existing utilities
- Over-engineering
- Under-engineering
- Missing type definitions
- Incomplete error types

**Focus**: Does this fit the codebase's conventions?

## Output Format

```markdown
# Code Review — {Feature/Branch}

> 审查时间：{date}
> 审查范围：{files/diffs reviewed}

## 总体评估

{Overall assessment: PASS | NEEDS CHANGES | BLOCKED}

## 发现问题

### 严重 (Critical)

Must fix before merge.

#### C1: {Title}

**文件**: `path/to/file.ts:line`
**轮次**: Round {N}

{Description of the issue}

**建议修复**:
{How to fix it}

---

### 重要 (Important)

Should fix before merge.

#### I1: {Title}

**文件**: `path/to/file.ts:line`
**轮次**: Round {N}

{Description of the issue}

**建议修复**:
{How to fix it}

---

### 建议 (Suggestion)

Consider fixing, but not blocking.

#### S1: {Title}

**文件**: `path/to/file.ts:line`
**轮次**: Round {N}

{Description of the suggestion}

---

## 各轮次摘要

| 轮次 | 发现问题数 | 严重 | 重要 | 建议 |
|------|-----------|------|------|------|
| 1. Logic | N | N | N | N |
| 2. Ordering | N | N | N | N |
| 3. Practices | N | N | N | N |
| 4. Security | N | N | N | N |
| 5. Magic Values | N | N | N | N |
| 6. Patterns | N | N | N | N |

## 积极方面

{What was done well — acknowledge good patterns}

## 建议的后续步骤

{What to do next based on the review}
```

## Key Rules

1. **Narrow scope**: Review files or diffs, not entire features
2. **Be specific**: Include file paths and line numbers
3. **Suggest fixes**: Don't just point out problems — show how to fix them
4. **Acknowledge good work**: Not everything needs criticism
5. **Prioritize**: Critical issues first, then important, then suggestions

## Severity Levels

- **Critical**: Must fix. Security vulnerabilities, data loss, corruption.
- **Important**: Should fix. Bugs, bad practices, maintainability issues.
- **Suggestion**: Consider fixing. Style improvements, minor optimizations.

## Before Merging

A PR should not be merged until:
- All Critical issues are resolved
- All Important issues are resolved or acknowledged
- The overall assessment is PASS or NEEDS CHANGES (not BLOCKED)
