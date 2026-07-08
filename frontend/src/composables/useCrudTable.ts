// 通用 CRUD 表格组合式函数
// 封装 list / create / edit / delete + 编辑对话框 + loading/submitting 状态
// 适用于管理员后台以「表格 + 弹窗表单」方式管理单一资源的场景（如原价表）
import { ref, reactive } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'

// 字段定义：驱动弹窗表单渲染、默认值与必填校验
export interface FieldDef {
  prop: string
  label: string
  type: 'input' | 'number' | 'switch'
  required?: boolean
  width?: number
  defaultValue?: string | number | boolean
}

// 资源访问适配器：把具体 API 客户端适配为通用 CRUD 接口
// ID 为主键类型（如 AdminResourceId），getId 返回 ID | null | undefined，
// update/remove 仅接收非空 ID（调用方在 id != null 后才调用）
export interface CrudResource<T, ID> {
  fetch: () => Promise<T[]>
  create: (payload: Record<string, unknown>) => Promise<unknown>
  update: (id: ID, payload: Record<string, unknown>) => Promise<unknown>
  remove: (id: ID) => Promise<unknown>
  // 从行数据取主键（编辑/删除时使用）；row 为 null 时返回 null/undefined 表示新增
  getId: (row: T | null) => ID | null | undefined
}

/**
 * useCrudTable 通用 CRUD 表格
 * @param resource 资源访问适配器
 * @param fields   字段定义（弹窗表单与校验）
 * @param entityLabel 实体中文名（用于弹窗标题与删除确认，如「原价记录」）
 */
export function useCrudTable<T extends Record<string, any>, ID = unknown>(
  resource: CrudResource<T, ID>,
  fields: FieldDef[],
  entityLabel: string
) {
  const loading = ref(false)
  const list = ref<T[]>([])
  const dialogVisible = ref(false)
  const dialogTitle = ref('')
  const editingRow = ref<T | null>(null)
  const formData = reactive<Record<string, any>>({})
  const submitting = ref(false)

  async function load() {
    loading.value = true
    try {
      list.value = await resource.fetch()
    } catch {
      list.value = []
    } finally {
      loading.value = false
    }
  }

  function resetForm() {
    Object.keys(formData).forEach((k) => delete formData[k])
  }

  // 按 fields 配置填充默认值
  function applyDefaults() {
    for (const f of fields) {
      if (f.defaultValue !== undefined) {
        formData[f.prop] = f.defaultValue
      } else if (f.type === 'switch') {
        formData[f.prop] = true
      } else if (f.type === 'number') {
        formData[f.prop] = 0
      } else {
        formData[f.prop] = ''
      }
    }
  }

  function openCreate() {
    editingRow.value = null
    dialogTitle.value = `新增${entityLabel}`
    resetForm()
    applyDefaults()
    dialogVisible.value = true
  }

  function openEdit(row: T) {
    editingRow.value = row
    dialogTitle.value = `编辑${entityLabel}`
    resetForm()
    Object.assign(formData, row)
    dialogVisible.value = true
  }

  // 提交：先做必填校验，再依据主键判断新增/更新
  async function submit() {
    for (const f of fields) {
      if (f.required) {
        const v = formData[f.prop]
        if (v == null || v === '') {
          ElMessage.warning(`请填写${f.label}`)
          return
        }
      }
    }
    submitting.value = true
    try {
      const payload: Record<string, unknown> = { ...formData }
      const id = resource.getId(editingRow.value)
      if (id != null) {
        await resource.update(id, payload)
        ElMessage.success('更新成功')
      } else {
        await resource.create(payload)
        ElMessage.success('创建成功')
      }
      dialogVisible.value = false
      await load()
    } catch {
      // 拦截器已提示
    } finally {
      submitting.value = false
    }
  }

  // 删除：二次确认后调用 remove，成功后重载
  async function remove(row: T) {
    const id = resource.getId(row)
    if (id == null) return
    try {
      await ElMessageBox.confirm(`确定删除该${entityLabel}？`, '删除确认', { type: 'warning' })
      await resource.remove(id)
      ElMessage.success('已删除')
      await load()
    } catch {
      // 用户取消或拦截器已提示
    }
  }

  return {
    loading,
    list,
    dialogVisible,
    dialogTitle,
    editingRow,
    formData,
    submitting,
    load,
    openCreate,
    openEdit,
    submit,
    remove
  }
}
