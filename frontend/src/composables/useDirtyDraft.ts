// 通用「脏草稿」组合式函数
// 封装 original / draft 双副本 + dirty 检测 + 仅保存变更项 + 重置
// 适用于管理员后台「就地编辑 + 批量保存变更项」场景（如算法参数 5 个分区）
//
// 设计要点：
// - original 与 draft 为独立浅拷贝，编辑 draft 不会污染 original
// - dirty 检测与保存均支持 filter，用于共享底层数组但按前缀分区的场景
//   （如 coefficient_configs 中 kc_ 前缀与非 kc_ 前缀分属两个分区，但共用一条 draft）
// - save 不内置 loading 态，由调用方持有独立 savingXxx ref，保留各分区按钮独立 loading 的精确行为
import { ref, type Ref } from 'vue'
import { ElMessage } from 'element-plus'

export interface DirtyDraftOptions<T> {
  // 标识同一项（按 id 或 key 在 original 与 draft 间匹配）
  identity: (item: T) => string | number
  // 两项是否等值：返回 true 视为无变化（dirty 检测用）
  equals: (a: T, b: T) => boolean
}

export function useDirtyDraft<T>(opts: DirtyDraftOptions<T>) {
  // 用 Ref<T[]> 显式标注（经 unknown 断言），规避 Vue 对泛型 T 的 UnwrapRefSimple 推断问题；
  // 运行时仍为深响应 ref，v-model 直接编辑 draft 元素属性可正常触发响应。
  const original = ref([]) as unknown as Ref<T[]>
  const draft = ref([]) as unknown as Ref<T[]>

  function clone(items: T[]): T[] {
    return items.map((x) => ({ ...x }))
  }

  // 装载服务器数据：original 与 draft 各持一份独立拷贝
  function setAll(items: T[]) {
    original.value = clone(items)
    draft.value = clone(items)
  }

  function clear() {
    original.value = []
    draft.value = []
  }

  // 是否存在变更；filter 可限定只检查子集（如 kc_ 前缀）
  function isDirty(filter?: (item: T) => boolean): boolean {
    const d = filter ? draft.value.filter(filter) : draft.value
    const o = filter ? original.value.filter(filter) : original.value
    if (d.length !== o.length) return true
    return d.some((item) => {
      const match = o.find((x) => opts.identity(x) === opts.identity(item))
      return !match || !opts.equals(match, item)
    })
  }

  // 返回变更项列表
  function getDirty(filter?: (item: T) => boolean): T[] {
    const d = filter ? draft.value.filter(filter) : draft.value
    return d.filter((item) => {
      const match = original.value.find((x) => opts.identity(x) === opts.identity(item))
      return !match || !opts.equals(match, item)
    })
  }

  /**
   * 保存变更项
   * @returns true 表示已成功保存（调用方应 reload 数据）；false 表示无变更或校验失败或持久化失败
   */
  async function save(params: {
    filter?: (item: T) => boolean
    persist: (item: T) => Promise<void>
    successLabel: (count: number) => string
    // 单项校验：返回非空字符串则 ElMessage.warning 并中止
    validate?: (item: T) => string | void
  }): Promise<boolean> {
    const dirty = getDirty(params.filter)
    if (dirty.length === 0) {
      ElMessage.info('无变更')
      return false
    }
    if (params.validate) {
      for (const item of dirty) {
        const err = params.validate(item)
        if (err) {
          ElMessage.warning(err)
          return false
        }
      }
    }
    try {
      await Promise.all(dirty.map(params.persist))
      ElMessage.success(params.successLabel(dirty.length))
      return true
    } catch {
      // 拦截器已提示
      return false
    }
  }

  // 重置 draft 到 original；filter 可限定只重置子集（其余行保持 draft 现值）
  function reset(filter?: (item: T) => boolean) {
    if (!filter) {
      draft.value = clone(original.value)
      return
    }
    const origMap = new Map(
      original.value.filter(filter).map((x) => [opts.identity(x), { ...x }] as [string | number, T])
    )
    draft.value = draft.value.map((item) => {
      if (filter(item)) {
        return origMap.get(opts.identity(item)) ?? ({ ...item } as T)
      }
      return item
    })
  }

  return { original, draft, setAll, clear, isDirty, getDirty, save, reset }
}
