# Tasks — 职位广场学员端

> 基于 issues.md | 创建：2026-09-01

## Task 1：新增 `JobListParams` 类型

- **Issue**：#1
- **文件**：`api/job.uts`
- **状态**：- [ ] 待完成

在 `api/job.uts` 中 `JobPosting` 类型后新增：

```typescript
export type JobListParams = {
    page ?: number
    page_size ?: number
    specialty_id ?: number
    region ?: string
    salary_min ?: number
    salary_max ?: number
    experience ?: string
}
```

---

## Task 2：新增 `JobListResult` 类型

- **Issue**：#1
- **文件**：`api/job.uts`
- **前置**：Task 1
- **状态**：- [ ] 待完成

```typescript
export type JobListResult = {
    items : JobPosting[]
    total : number
}
```

---

## Task 3：实现 `getJobListApi()` 函数

- **Issue**：#1
- **文件**：`api/job.uts`
- **前置**：Task 1, Task 2
- **状态**：- [ ] 待完成

实现要点：
- 参数为 `JobListParams`（默认空对象）
- 构造 query string，跳过 null/undefined/空值
- 调用 `get('/jobs?' + qs)`
- 解析响应 `items` 数组，每项映射为 `JobPosting`
- 返回 `JobListResult`

参考实现见 `plan.md` Task 1 代码段。

---

## Task 4：删除 mock 数据

- **Issue**：#2
- **文件**：`pages/jobs/job-list.uvue`
- **前置**：无
- **状态**：- [ ] 待完成

删除内容：
- `type JobItem` 定义（第 81-89 行）
- `const jobs = ref<JobItem[]>([...])` 硬编码数组（第 91-100 行）

---

## Task 5：引入真实 API 和类型

- **Issue**：#2
- **文件**：`pages/jobs/job-list.uvue`
- **前置**：Task 4
- **状态**：- [ ] 待完成

新增 import：

```typescript
import { getJobListApi } from '../../api/job'
import type { JobPosting } from '../../api/job'
```

替换 state 类型：

```typescript
const jobs = ref<JobPosting[]>([])
```

---

## Task 6：实现 `loadJobs(reset)` 函数

- **Issue**：#2, #4
- **文件**：`pages/jobs/job-list.uvue`
- **前置**：Task 5
- **状态**：- [ ] 待完成

实现要点：
- `reset=true` 时重置 page=1、noMore=false、jobs=[]
- 防重复加载（`loading` 锁）
- 调用 `getJobListApi({ page, page_size: 20, specialty_id })`
- `reset=true` 赋值，`reset=false` 追加
- 判断结束条件：`items.length < 20 || items.length >= total`
- 错误时 toast 提示

---

## Task 7：模板字段对齐

- **Issue**：#2
- **文件**：`pages/jobs/job-list.uvue`
- **前置**：Task 5
- **状态**：- [ ] 待完成

模板变更：
- `job.company` → `job.company_name`
- `job.salary` → `formatSalary(job)`（新增格式化函数）
- `job.location` → `job.region`
- 删除 `job.jobType` 相关的 `<view class="job-type-tag">`
- `job.description` 加 `numberOfLines="2"` 截断

新增 `formatSalary()` 辅助函数：

```typescript
function formatSalary(job : JobPosting) : string {
    if (job.salary_text.length > 0) return job.salary_text
    if (job.salary_min != null && job.salary_max != null) {
        return job.salary_min!.toString() + '-' + job.salary_max!.toString() + ' 元/月'
    }
    if (job.salary_min != null) return job.salary_min!.toString() + ' 元/月起'
    return '面议'
}
```

---

## Task 8：`onLoad` 调用 `loadJobs(true)`

- **Issue**：#2
- **文件**：`pages/jobs/job-list.uvue`
- **前置**：Task 6
- **状态**：- [ ] 待完成

```typescript
import { onLoad } from '@dcloudio/uni-app'

onLoad(() => {
    loadJobs(true)
})
```

---

## Task 9：筛选标签动态加载

- **Issue**：#3
- **文件**：`pages/jobs/job-list.uvue`
- **前置**：Task 5
- **状态**：- [ ] 待完成

实现要点：
- 引入 `getCatalogTreeSpecialties()` from `../../api/course`
- 新增 `filterTags` ref，类型 `{ label: string, id: number }[]`
- 新增 `loadFilters()` 函数，调用 API 构建标签数组
- `onLoad` 中同时调用 `loadFilters()` 和 `loadJobs(true)`
- `onFilterChange(idx)` 切换后调用 `loadJobs(true)`
- 失败时降级为仅 `[{ label: '全部', id: 0 }]`

---

## Task 10：分页加载与 loading 态

- **Issue**：#4
- **文件**：`pages/jobs/job-list.uvue`
- **前置**：Task 6
- **状态**：- [ ] 待完成

确认模板中已有的分页 UI 生效：
- `@scrolltolower="onLoadMore"` → 调用 `loadJobs(false)`
- `v-if="loading"` 的加载中提示
- `v-if="noMore && jobs.length > 0"` 的「没有更多了」提示
- `v-if="jobs.length == 0 && !loading"` 的空状态

---

## Task 11：全流程验证

- **Issue**：#5
- **前置**：Task 1 ~ Task 10
- **状态**：- [ ] 待完成

| # | 步骤 | 预期 | 通过 |
|---|------|------|------|
| 1 | 进入职位广场 | 列表显示真实数据 | - [ ] |
| 2 | 切换筛选标签 | 列表刷新过滤 | - [ ] |
| 3 | 滚动到底部 | 自动加载下一页 | - [ ] |
| 4 | 加载完毕 | 显示「没有更多了」 | - [ ] |
| 5 | 点击卡片 | 跳转详情页 | - [ ] |
| 6 | 详情页举报 | 按钮变「已举报」 | - [ ] |
| 7 | 详情页投递 | 按钮变「已投递」 | - [ ] |
| 8 | 无数据时 | 显示空状态 | - [ ] |
