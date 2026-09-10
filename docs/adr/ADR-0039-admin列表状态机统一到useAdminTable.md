# ADR-0039: admin 列表状态机统一到 useAdminTable

- 状态：已接受（2026-09-10）
- 关联：#766 后续规划、ADR-0015（useAdminTable 原始决策）、#439（useAsyncPage 三态 + 分页收编）
- 前置：ADR-0037、ADR-0038

## 背景

admin 域 19 个页面长期存在**两套并存的列表状态机**，外加 7 页完全手写：

| 状态机 | 覆盖面 | 三态 | 分页 | 搜索/筛选 | 行操作 |
|---|---|---|---|---|---|
| `useAsyncPage`（#439） | 全站 40 处，admin 6 页 | loading / **error / retry** | ✓ | 页面自管 | 页面自管 |
| `useAdminTable`（ADR-0015） | admin 6 页 | loading（无 error/retry） | ✓ | **内置 filters + search** | **内置 actions 分发 + confirmDelete** |
| 手写 | admin 7 页 | 各自实现 | 各自实现 | — | — |

两套功能重叠但不等价：admin 列表的常态是「搜索 + 分页 + 行操作（含删除确认）」，
`useAdminTable` 恰好内置这三样，而 `useAsyncPage` 只提供三态与分页。

## 决策

**统一到 `useAdminTable`**，分三步一次完成：

1. **先补能力**：给 `useAdminTable` 补 `error` + `retry`（对齐 `useAsyncPage` 的三态语义）。
   这是迁移的前提 —— 现有实现只有 `loading`，直接迁移会丢掉 #439 引入的错误重试能力。
2. **再迁 13 页**：6 个已用 `useAsyncPage` 的 admin 页 + 7 个手写页，全部改用 `useAdminTable`。
3. **纯函数与驳回弹窗另批**：见下节。

### 为什么不用全站事实标准的 useAsyncPage

- `useAsyncPage` 服务于全站 40 处（学员端/讲师端/招聘端等），是**通用**三态 + 分页件。
  让它长出行操作分发与删除确认，等于把 admin 的组织语义灌进通用件。
- `useAdminTable` 是 **admin 专属**且已有 ADR-0015 背书，内置 `fetch` adapter + `actions`
  adapter 的心智与 admin 页面形态一致（页面只声明两个 adapter）。
- 因此分工是：**admin 列表用 `useAdminTable`，其余场景继续用 `useAsyncPage`**（40 处不动）。

### 为什么一次迁完而不是分批试点

用户拍板一次完成。代价是回归面大（13 页），收益是避免双轨长期共存的维护成本。
每页迁移都是「替换状态来源、保留模板与业务语义」，可脚本化 + 逐页验证。

## 边界（明确不做）

- **不改 `useAsyncPage`**：它不是 admin 的件，40 处调用不动
- **不统一三域审核状态机**：题库（draft/pending/published，驳回回 draft 可再编辑）、
  资料审核（整型 status，终态）、投稿（pending/approved/rejected/withdrawn/archived 五态）
  在数据层就不同构。三域**唯一同构的是 `reject_reason`**，这块另行抽取（见下节）
- **不抽「admin 通用列表页组件」**：字段与操作语义各异，prop 化会重蹈 UiFilterBar /
  UiTable 的覆辙（两者都明确拒绝 prop 化/封装）

## 配套：驳回理由弹窗（另一批）

三域审核共有的 `reject_reason` 抽为 `useRejectReasonDialog`：只管 `visible` / `reason` /
`submitting` 三件套与弹窗 UI 状态，**提交动作（单条 / 批量 / 详情页触发）由调用方注入**。
各域状态机与批量差异不动。

## 后果

- `useAdminTable` 成为 admin 列表页的**唯一**状态机，13 页行为对齐（三态 + 分页 + 搜索 + 行操作）
- admin 域出现可单测的共享逻辑层（此前 19 页仅 2 个专属组件），可测性显著提升
- 回滚成本：13 页改动，但每页改动局部、语义不变，可逐页回退

## 术语

「审核」在本仓库是**过载词**（三个不同状态机），「驳回理由」才是三域唯一同构概念 ——
已记入 `CONTEXT.md` 的「审核（review）」段，勿混用。
