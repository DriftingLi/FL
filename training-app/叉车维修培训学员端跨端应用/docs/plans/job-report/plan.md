# 职位举报 UI — 实施计划

> 创建时间：2026-09-01 | 更新时间：2026-09-01

## 现状总结

经过代码探查，发现 **大部分工作已完成**：

| 组件 | 状态 | 文件 |
|------|------|------|
| 后端举报 API | ✅ 已实现 | `backend/internal/api/job_report.go` |
| 后端职位列表 API | ✅ 已实现 | `backend/internal/api/job.go` → `GET /api/jobs` |
| 前端 API 封装 | ⚠️ 部分完成 | `api/job.uts` — 缺 `getJobListApi()` |
| 职位详情页 | ✅ 已完成 | `pages/jobs/job-detail.uvue` (547 行，含举报弹窗+投递) |
| 职位列表页 | ❌ 未接入 | `pages/jobs/job-list.uvue` — 仍为 mock 数据 |
| 路由注册 | ✅ 已完成 | `pages.json` 已配置 |

**结论：只需补全「列表页接入真实 API」即可闭环。**

## 待实施任务

### Task 1：`api/job.uts` 补充 `getJobListApi()`

在 `api/job.uts` 末尾新增列表查询函数，对接 `GET /api/jobs`。

```typescript
/** 职位列表查询参数 */
export type JobListParams = {
    page ?: number
    page_size ?: number
    specialty_id ?: number
    region ?: string
    salary_min ?: number
    salary_max ?: number
    experience ?: string
}

/** 职位列表结果 */
export type JobListResult = {
    items : JobPosting[]
    total : number
}

/**
 * 职位广场列表（学员侧，仅 open 且未强制下架）
 */
export function getJobListApi(params : JobListParams = {}) : Promise<JobListResult> {
    const query : UTSJSONObject = {} as UTSJSONObject
    if (params.page != null && params.page! > 0) query['page'] = params.page!.toString()
    if (params.page_size != null && params.page_size! > 0) query['page_size'] = params.page_size!.toString()
    if (params.specialty_id != null && params.specialty_id! > 0) query['specialty_id'] = params.specialty_id!.toString()
    if (params.region != null && params.region!.length > 0) query['region'] = params.region!
    if (params.salary_min != null) query['salary_min'] = params.salary_min!.toString()
    if (params.salary_max != null) query['salary_max'] = params.salary_max!.toString()
    if (params.experience != null && params.experience!.length > 0) query['experience'] = params.experience!

    const qs = Object.keys(query).map((k : string) : string => {
        return encodeURIComponent(k) + '=' + encodeURIComponent(query[k]!)
    }).join('&')
    const url = '/jobs' + (qs.length > 0 ? '?' + qs : '')

    return get(url).then((data : UTSJSONObject) : JobListResult => {
        const itemsRaw = data['items'] as UTSJSONObject[] ?? []
        const items : JobPosting[] = itemsRaw.map((obj : UTSJSONObject) : JobPosting => {
            return {
                id: obj['id'] as number,
                title: (obj['title'] as string) ?? '',
                region: (obj['region'] as string) ?? '',
                salary_min: obj['salary_min'] as number | undefined,
                salary_max: obj['salary_max'] as number | undefined,
                salary_text: (obj['salary_text'] as string) ?? '',
                experience_req: (obj['experience_req'] as string) ?? '',
                description: (obj['description'] as string) ?? '',
                specialty_id: obj['specialty_id'] as number | undefined,
                published_at: (obj['published_at'] as string) ?? '',
                company_name: (obj['company_name'] as string) ?? '',
                status: (obj['status'] as string) ?? 'open',
                forced_offline: (obj['forced_offline'] as boolean) ?? false,
            } as JobPosting
        })
        return {
            items: items,
            total: data['total'] as number,
        } as JobListResult
    })
}
```

### Task 2：重写 `pages/jobs/job-list.uvue` 接入真实 API

核心改动：
1. 删除 mock `JobItem` 类型和硬编码数组，改用 `JobPosting` + `getJobListApi()`
2. 筛选标签改为动态加载 specialty 列表（复用 `getCatalogTreeSpecialties()`），点击时传 `specialty_id` 筛选
3. 实现分页加载（`page` + `scrolltolower`）
4. 卡片字段对齐 `JobPosting` DTO（`company_name`/`salary_text`/`region`/`description`）

关键逻辑：

```typescript
import { ref, onMounted } from 'vue'
import { onLoad } from '@dcloudio/uni-app'
import { useStatusBarHeight } from '../../utils/system'
import { getJobListApi, REPORT_REASONS } from '../../api/job'
import { getCatalogTreeSpecialties } from '../../api/course'
import type { JobPosting } from '../../api/job'

// 状态
const jobs = ref<JobPosting[]>([])
const loading = ref<boolean>(false)
const noMore = ref<boolean>(false)
const page = ref<number>(1)
const PAGE_SIZE = 20

// 筛选
const filterTags = ref<{ label: string, id: number }[]>([])
const activeFilter = ref<number>(0)

onLoad(() => {
    loadFilters()
    loadJobs(true)
})

async function loadFilters() : Promise<void> {
    try {
        const specialties = await getCatalogTreeSpecialties()
        const tags = [{ label: '全部', id: 0 }]
        for (const s of specialties) {
            tags.push({ label: s.name, id: s.specialty_id })
        }
        filterTags.value = tags
    } catch (e) {
        // 降级：仅显示「全部」
    }
}

async function loadJobs(reset : boolean = false) : Promise<void> {
    if (loading.value) return
    if (reset) {
        page.value = 1
        noMore.value = false
        jobs.value = []
    }
    if (noMore.value) return
    loading.value = true
    try {
        const specialtyId = filterTags.value[activeFilter.value]?.id ?? 0
        const result = await getJobListApi({
            page: page.value,
            page_size: PAGE_SIZE,
            specialty_id: specialtyId > 0 ? specialtyId : undefined,
        })
        if (reset) {
            jobs.value = result.items
        } else {
            jobs.value = jobs.value.concat(result.items)
        }
        if (result.items.length < PAGE_SIZE || jobs.value.length >= result.total) {
            noMore.value = true
        }
        page.value++
    } catch (e) {
        uni.showToast({ title: '加载失败', icon: 'none' })
    } finally {
        loading.value = false
    }
}

function onFilterChange(idx : number) : void {
    activeFilter.value = idx
    loadJobs(true)
}

function onLoadMore() : void {
    loadJobs(false)
}
```

模板变更：
- `job.company` → `job.company_name`
- `job.salary` → `formatSalary(job)` (复用详情页的格式化逻辑)
- `job.location` → `job.region`
- 删除 `job.jobType`（后端无此字段）
- `job.description` 截断显示（过长时 `numberOfLines=2`）

### Task 3：验证

完成后需验证：

```bash
# 前端类型检查
cd frontend && npm run type-check  # 若项目有此脚本

# 手动验证
1. 职位列表页加载 → 应显示真实职位数据
2. 切换筛选标签 → 列表按专业方向过滤
3. 滚动到底 → 自动加载下一页
4. 点击职位卡片 → 跳转详情页，数据一致
5. 详情页举报 → 提交成功，按钮变为「已举报」
6. 详情页投递 → 投递成功，按钮变为「已投递」
```

## 文件变更清单

| 文件 | 操作 | 说明 |
|------|------|------|
| `api/job.uts` | 编辑 | 新增 `getJobListApi()`、`JobListParams`、`JobListResult` |
| `pages/jobs/job-list.uvue` | 重写 | mock → 真实 API + 分页 + 动态筛选 |

## 风险与注意事项

- 后端 `GET /api/jobs` 需要认证（`hrwai_user` 角色），确保请求头携带 JWT
- `getCatalogTreeSpecialties()` 返回的 specialty 列表与后端 `specialty_id` 对齐，但当前 API 返回的 `items` 中的 `specialty_id` 可能为 null（后端允许字典项删除时置空），前端需做 null 安全处理
- 列表页与详情页的 `JobPosting` 类型复用同一个，保持一致
