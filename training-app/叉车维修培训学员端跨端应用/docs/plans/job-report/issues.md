# Issues — 职位广场学员端

> 基于 PRD v1.0 | Spec #449 | 创建：2026-09-01

## Issue #1：`api/job.uts` 新增 `getJobListApi()`

- **类型**：feat
- **优先级**：P0
- **关联**：US-1, US-2, US-3, F3
- **文件**：`api/job.uts`

**描述**：新增职位列表查询函数，对接后端 `GET /api/jobs`。

**子任务**：

1. 新增 `JobListParams` 类型（page/page_size/specialty_id/region/salary_min/salary_max/experience，全部可选）
2. 新增 `JobListResult` 类型（items: JobPosting[], total: number）
3. 实现 `getJobListApi(params)` — 构造 query string，调用 `get()`，解析响应为 `JobListResult`
4. 复用已有 `JobPosting` 类型，确保字段与后端 DTO 对齐

**验收**：函数可被 import，TS 类型检查通过。

---

## Issue #2：`job-list.uvue` 接入真实 API

- **类型**：feat
- **优先级**：P0
- **关联**：US-1, US-4, F1
- **前置**：Issue #1
- **文件**：`pages/jobs/job-list.uvue`

**描述**：删除 mock 数据，改用 `getJobListApi()` 加载真实职位列表。

**子任务**：

1. 删除 `JobItem` 类型和硬编码 `jobs` 数组
2. 引入 `JobPosting` 类型和 `getJobListApi()`
3. 实现 `loadJobs(reset)` — 支持首次加载与增量加载
4. 模板字段对齐：`job.company_name`、`formatSalary(job)`、`job.region`
5. 空状态 / 加载中 / 没有更多了 三态 UI

**验收**：AC-1, AC-5, AC-8。

---

## Issue #3：`job-list.uvue` 筛选标签动态化

- **类型**：feat
- **优先级**：P0
- **关联**：US-2, F1
- **前置**：Issue #2
- **文件**：`pages/jobs/job-list.uvue`

**描述**：筛选标签从硬编码改为动态加载 specialty 列表。

**子任务**：

1. 引入 `getCatalogTreeSpecialties()`（来自 `api/course.uts`）
2. 页面加载时调用，构建 `[{ label: '全部', id: 0 }, ...]` 数组
3. 点击标签时传 `specialty_id` 参数重新加载列表
4. 加载失败降级为仅显示「全部」

**验收**：AC-2。

---

## Issue #4：`job-list.uvue` 分页加载

- **类型**：feat
- **优先级**：P0
- **关联**：US-3, F1
- **前置**：Issue #2
- **文件**：`pages/jobs/job-list.uvue`

**描述**：实现 scroll-view 分页加载。

**子任务**：

1. 维护 `page` 计数器和 `noMore` 标记
2. `scrolltolower` 事件触发 `loadJobs(false)` 增量追加
3. 响应 `items.length < PAGE_SIZE` 或 `items.length >= total` 时置 `noMore = true`
4. 防重复加载（`loading` 锁）

**验收**：AC-3, AC-4。

---

## Issue #5：集成验证

- **类型**：test
- **优先级**：P1
- **关联**：AC-1 ~ AC-8
- **前置**：Issue #1 ~ #4

**描述**：全流程手动验证。

**验证清单**：

| # | 步骤 | 预期 |
|---|------|------|
| 1 | 进入职位广场 | 列表显示真实数据，按发布时间倒序 |
| 2 | 切换筛选标签 | 列表刷新为对应方向 |
| 3 | 滚动到底部 | 自动加载下一页 |
| 4 | 所有加载完 | 底部显示「没有更多了」 |
| 5 | 点击卡片 | 跳转详情页，数据一致 |
| 6 | 详情页举报 | 提交成功，按钮变「已举报」 |
| 7 | 详情页投递 | 提交成功，按钮变「已投递」 |
| 8 | 无数据时 | 显示空状态占位图 |
