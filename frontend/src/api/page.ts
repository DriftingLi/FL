// 分页容器（ADR-0060 决策 6 / 票 6）：`Page<T>` 是 **api 层**的出口形状，不是页面的。
//
// 后端对「这一页的行」有 9+ 个键名（items / list / topics / reports / questions / tutors /
// requests / courses / records …，见 `api/generated/*`）。键方言此前被 11 页 17 处
// `{ list: res.items || [], total: res.total || 0 }` 手抄在页面里——页面因此必须知道每个域
// 的键叫什么，而它本不该知道。归位后：**各域列表函数在自己的 api 模块出口即返回 `Page<T>`**
// （那里本就写着生成物类型，键方言在离生成物最近的地方消解），页面的 `fetch` adapter
// 退化成一行 `return someApi.listX(...)`。
//
// 依赖方向是单向的：`composables/` 与 `pages/` import 本文件，本文件不 import 任何上层
// （ADR-0060 被否备选：让 api 层反向 import composable；也不在 `client.ts` 嗅探键名——
// 那会把「哪个键算列表」从类型降为运行期约定，新域换键名会静默返回空列表）。
//
// 本模块只留一个构造器 `toPage`（不搞一族）：键名由调用方交出、两个 `|| []` / `|| 0`
// 兜底在此单点实现。
//
// 锁：`api/__tests__/page.spec.ts` —— `pages/` 与 `components/` 里不得再有 `useAdminTable`
// 的 `fetch` adapter 手搭分页容器。

/** 中立分页容器：api 层列表函数的统一出口（键方言在它之前就被消解）。 */
export interface Page<T> {
  items: T[]
  total: number
}

/**
 * 生成物分页负载 → `Page<T>` 的唯一归一处。
 *
 * 两个参数都由调用方（api 模块）从**自己那条端点的生成物**上取：行数组用本域的键
 * （`res.topics` / `data.tutors` / …），总数一律是 `total`。兜底口径与此前页面里的
 * `|| []` / `|| 0` 逐字等价：行缺失或非数组 → `[]`，total 缺失 / null / 非有限值 → 0。
 */
export function toPage<T>(
  items: T[] | null | undefined,
  total: number | null | undefined
): Page<T> {
  return {
    items: Array.isArray(items) ? items : [],
    total: typeof total === 'number' && Number.isFinite(total) ? total : 0
  }
}
