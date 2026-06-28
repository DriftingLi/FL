<script setup lang="ts">
// 残值评估配置管理（管理员）
// 使用 el-tabs 组织 10 个配置板块：原价表 / 品牌类型 / 品牌 / 车辆类型 / 系列·吨位 / 配置·门架 / 电池类型 / 车况评级 / 区域系数 / 算法参数
import { ref, reactive, onMounted, watch } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Plus, Edit, Delete, Refresh } from '@element-plus/icons-vue'
import PageHeader from '@/components/valuation/PageHeader.vue'
import {
  adminResources,
  listAdminCoefficients,
  updateAdminCoefficients,
  type AdminRow,
  type AdminResourceKey
} from '@/api/valuation/admin'
import type { CoefficientConfig } from '@/types/valuation/evaluation'

// ========== 通用 Tab 状态 ==========
interface TabState {
  loading: boolean
  list: AdminRow[]
}

const tabStates = reactive<Record<AdminResourceKey, TabState>>({
  originalPrices: { loading: false, list: [] },
  brandTypes: { loading: false, list: [] },
  brands: { loading: false, list: [] },
  vehicleTypes: { loading: false, list: [] },
  series: { loading: false, list: [] },
  tonnages: { loading: false, list: [] },
  configTypes: { loading: false, list: [] },
  mastTypes: { loading: false, list: [] },
  mastHeights: { loading: false, list: [] },
  batteryTypes: { loading: false, list: [] },
  conditionRatings: { loading: false, list: [] },
  regionCoefficients: { loading: false, list: [] }
})

// 当前激活的 tab
const activeTab = ref<string>('originalPrices')

// 加载指定资源的列表
async function loadList(key: AdminResourceKey) {
  const state = tabStates[key]
  state.loading = true
  try {
    state.list = await adminResources[key].list()
  } catch {
    // 拦截器已提示
    state.list = []
  } finally {
    state.loading = false
  }
}

// 切换 tab 时懒加载
function onTabChange(name: string) {
  const key = name as AdminResourceKey
  if (tabStates[key].list.length === 0) {
    loadList(key)
  }
}

onMounted(() => {
  loadList('originalPrices')
})

// ========== 通用编辑对话框 ==========
const dialogVisible = ref(false)
const dialogTitle = ref('')
const editingKey = ref<AdminResourceKey | null>(null)
const editingRow = ref<AdminRow | null>(null)
const formData = reactive<AdminRow>({})
const submitting = ref(false)

/** 资源的中文名 + 字段配置（用于动态渲染表单与表格列） */
interface FieldDef {
  prop: string
  label: string
  /** 表单输入类型 */
  type: 'input' | 'number' | 'switch'
  required?: boolean
  /** 表格列宽度 */
  width?: number
}

interface ResourceSchema {
  title: string
  fields: FieldDef[]
}

const SCHEMAS: Record<AdminResourceKey, ResourceSchema> = {
  originalPrices: {
    title: '原价表',
    fields: [
      { prop: 'brand_type', label: '品牌类型', type: 'input', required: true, width: 140 },
      { prop: 'brand', label: '品牌', type: 'input', required: true, width: 140 },
      { prop: 'vehicle_type', label: '车辆类型', type: 'input', required: true, width: 140 },
      { prop: 'series', label: '系列', type: 'input', width: 140 },
      { prop: 'tonnage', label: '吨位', type: 'number', width: 100 },
      { prop: 'original_price', label: '原价（万元）', type: 'number', required: true, width: 140 }
    ]
  },
  brandTypes: {
    title: '品牌类型',
    fields: [
      { prop: 'name', label: '名称', type: 'input', required: true, width: 200 },
      { prop: 'k_type', label: 'K_type 系数', type: 'number', required: true, width: 140 }
    ]
  },
  brands: {
    title: '品牌',
    fields: [
      { prop: 'name', label: '名称', type: 'input', required: true, width: 160 },
      { prop: 'brand_type', label: '品牌类型', type: 'input', required: true, width: 140 },
      { prop: 'k_brand', label: 'K_brand 系数', type: 'number', required: true, width: 140 },
      { prop: 'is_active', label: '启用', type: 'switch', width: 100 }
    ]
  },
  vehicleTypes: {
    title: '车辆类型',
    fields: [
      { prop: 'name', label: '名称', type: 'input', required: true, width: 200 },
      {
        prop: 'power_type',
        label: '动力类型',
        type: 'input',
        required: true,
        width: 140
      }
    ]
  },
  series: {
    title: '系列',
    fields: [
      { prop: 'brand', label: '品牌', type: 'input', required: true, width: 160 },
      { prop: 'name', label: '系列名称', type: 'input', required: true, width: 200 }
    ]
  },
  tonnages: {
    title: '吨位',
    fields: [{ prop: 'value', label: '吨位值', type: 'number', required: true, width: 200 }]
  },
  configTypes: {
    title: '配置类型',
    fields: [{ prop: 'name', label: '名称', type: 'input', required: true, width: 240 }]
  },
  mastTypes: {
    title: '门架类型',
    fields: [{ prop: 'name', label: '名称', type: 'input', required: true, width: 240 }]
  },
  mastHeights: {
    title: '门架高度',
    fields: [{ prop: 'value_mm', label: '高度（mm）', type: 'number', required: true, width: 240 }]
  },
  batteryTypes: {
    title: '电池类型',
    fields: [{ prop: 'name', label: '名称', type: 'input', required: true, width: 240 }]
  },
  conditionRatings: {
    title: '车况评级',
    fields: [
      { prop: 'rating', label: '评级', type: 'input', required: true, width: 120 },
      { prop: 'label', label: '中文标签', type: 'input', required: true, width: 160 },
      { prop: 'base_coefficient', label: '基础系数', type: 'number', required: true, width: 140 }
    ]
  },
  regionCoefficients: {
    title: '区域系数',
    fields: [
      { prop: 'province', label: '省份', type: 'input', required: true, width: 140 },
      { prop: 'city', label: '城市', type: 'input', required: true, width: 140 },
      { prop: 'coefficient', label: '系数', type: 'number', required: true, width: 140 }
    ]
  }
}

/** 表格列：包含 id + 各字段 + 操作列 */
function tableColumns(key: AdminResourceKey): FieldDef[] {
  return [{ prop: 'id', label: 'ID', type: 'number', width: 70 }, ...SCHEMAS[key].fields]
}

/** 打开新增对话框 */
function openCreate(key: AdminResourceKey) {
  editingKey.value = key
  editingRow.value = null
  dialogTitle.value = `新增${SCHEMAS[key].title}`
  // 重置表单数据
  Object.keys(formData).forEach((k) => delete formData[k])
  // 设置默认值
  for (const f of SCHEMAS[key].fields) {
    if (f.type === 'switch') formData[f.prop] = true
    else if (f.type === 'number') formData[f.prop] = 0
    else formData[f.prop] = ''
  }
  dialogVisible.value = true
}

/** 打开编辑对话框 */
function openEdit(key: AdminResourceKey, row: AdminRow) {
  editingKey.value = key
  editingRow.value = row
  dialogTitle.value = `编辑${SCHEMAS[key].title}`
  // 重置并复制行数据
  Object.keys(formData).forEach((k) => delete formData[k])
  Object.assign(formData, row)
  dialogVisible.value = true
}

/** 提交新增/编辑 */
async function handleSubmit() {
  if (!editingKey.value) return
  const key = editingKey.value
  // 必填校验
  for (const f of SCHEMAS[key].fields) {
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
    if (editingRow.value?.id) {
      await adminResources[key].update(editingRow.value.id, payload)
      ElMessage.success('更新成功')
    } else {
      await adminResources[key].create(payload)
      ElMessage.success('创建成功')
    }
    dialogVisible.value = false
    await loadList(key)
  } catch {
    // 拦截器已提示
  } finally {
    submitting.value = false
  }
}

/** 删除 */
async function handleDelete(key: AdminResourceKey, row: AdminRow) {
  if (!row.id) return
  try {
    await ElMessageBox.confirm(`确定删除该${SCHEMAS[key].title}记录？`, '删除确认', {
      type: 'warning'
    })
    await adminResources[key].remove(row.id)
    ElMessage.success('已删除')
    await loadList(key)
  } catch {
    // 用户取消或拦截器已提示
  }
}

// ========== 算法参数（Tab 10）：独立表单 ==========
const coefficients = ref<CoefficientConfig[]>([])
const coefficientsLoading = ref(false)
const coefficientsSaving = ref(false)
// 本地编辑副本（避免直接修改原数据）
const coefficientsDraft = ref<CoefficientConfig[]>([])

async function loadCoefficients() {
  coefficientsLoading.value = true
  try {
    coefficients.value = await listAdminCoefficients()
    // 深拷贝作为编辑副本
    coefficientsDraft.value = coefficients.value.map((c) => ({ ...c }))
  } catch {
    coefficients.value = []
    coefficientsDraft.value = []
  } finally {
    coefficientsLoading.value = false
  }
}

async function saveCoefficients() {
  coefficientsSaving.value = true
  try {
    const updated = await updateAdminCoefficients(coefficientsDraft.value)
    coefficients.value = updated
    coefficientsDraft.value = updated.map((c) => ({ ...c }))
    ElMessage.success('算法参数已保存')
  } catch {
    // 拦截器已提示
  } finally {
    coefficientsSaving.value = false
  }
}

/** Tab 切换到算法参数时加载 */
watch(activeTab, (name) => {
  if (name === 'coefficients' && coefficients.value.length === 0) {
    loadCoefficients()
  }
})

// ========== Tab 配置 ==========
const tabs: Array<{ name: AdminResourceKey | 'coefficients'; label: string }> = [
  { name: 'originalPrices', label: '原价表' },
  { name: 'brandTypes', label: '品牌类型' },
  { name: 'brands', label: '品牌' },
  { name: 'vehicleTypes', label: '车辆类型' },
  { name: 'series', label: '系列' },
  { name: 'tonnages', label: '吨位' },
  { name: 'configTypes', label: '配置类型' },
  { name: 'mastTypes', label: '门架类型' },
  { name: 'mastHeights', label: '门架高度' },
  { name: 'batteryTypes', label: '电池类型' },
  { name: 'conditionRatings', label: '车况评级' },
  { name: 'regionCoefficients', label: '区域系数' },
  { name: 'coefficients', label: '算法参数' }
]
</script>

<template>
  <div class="config-manage valuation-root">
    <div class="app-container">
      <PageHeader title="残值评估配置" subtitle="valuation config">
        <template #actions>
          <el-button
            :icon="Refresh"
            @click="activeTab === 'coefficients' ? loadCoefficients() : loadList(activeTab as AdminResourceKey)"
          >
            刷新当前
          </el-button>
        </template>
      </PageHeader>

      <el-tabs v-model="activeTab" type="border-card" @tab-change="onTabChange">
        <!-- 通用资源 Tab（原价表 / 品牌类型 / ... / 区域系数） -->
        <el-tab-pane
          v-for="tab in tabs.filter((t) => t.name !== 'coefficients')"
          :key="tab.name"
          :label="tab.label"
          :name="tab.name"
        >
          <div class="tab-toolbar">
            <el-button type="primary" :icon="Plus" @click="openCreate(tab.name as AdminResourceKey)">
              新增
            </el-button>
          </div>
          <el-table
            v-loading="tabStates[tab.name as AdminResourceKey].loading"
            :data="tabStates[tab.name as AdminResourceKey].list"
            stripe
            border
            style="width: 100%"
            empty-text="暂无数据"
          >
            <el-table-column
              v-for="col in tableColumns(tab.name as AdminResourceKey)"
              :key="col.prop"
              :prop="col.prop"
              :label="col.label"
              :width="col.width"
              align="center"
            >
              <template #default="{ row }">
                <el-tag v-if="col.type === 'switch'" :type="row[col.prop] ? 'success' : 'info'">
                  {{ row[col.prop] ? '启用' : '禁用' }}
                </el-tag>
                <span v-else>{{ row[col.prop] ?? '-' }}</span>
              </template>
            </el-table-column>
            <el-table-column label="操作" width="160" fixed="right" align="center">
              <template #default="{ row }">
                <el-button type="primary" link size="small" :icon="Edit" @click="openEdit(tab.name as AdminResourceKey, row)">
                  编辑
                </el-button>
                <el-button type="danger" link size="small" :icon="Delete" @click="handleDelete(tab.name as AdminResourceKey, row)">
                  删除
                </el-button>
              </template>
            </el-table-column>
          </el-table>
        </el-tab-pane>

        <!-- 算法参数 Tab（GET + PUT，无 CRUD） -->
        <el-tab-pane label="算法参数" name="coefficients">
          <div class="tab-toolbar">
            <span class="coef-tip">编辑后点击「保存」整体提交（PUT 全量替换）</span>
            <el-button type="primary" :loading="coefficientsSaving" @click="saveCoefficients">
              保存
            </el-button>
          </div>
          <el-table
            v-loading="coefficientsLoading"
            :data="coefficientsDraft"
            stripe
            border
            style="width: 100%"
            empty-text="暂无参数"
          >
            <el-table-column prop="key" label="参数键" width="200" />
            <el-table-column label="参数值" width="200">
              <template #default="{ row }">
                <el-input-number
                  v-model="row.value"
                  :step="0.01"
                  :precision="4"
                  :min="0"
                  :max="10"
                  style="width: 100%"
                />
              </template>
            </el-table-column>
            <el-table-column prop="description" label="描述" min-width="240" />
          </el-table>
        </el-tab-pane>
      </el-tabs>

      <!-- 通用编辑对话框 -->
      <el-dialog
        v-model="dialogVisible"
        :title="dialogTitle"
        width="560px"
        destroy-on-close
      >
        <el-form :model="formData" label-width="120px">
          <el-form-item
            v-for="f in (editingKey ? SCHEMAS[editingKey].fields : [])"
            :key="f.prop"
            :label="f.label"
            :required="f.required"
          >
            <el-input
              v-if="f.type === 'input'"
              v-model="formData[f.prop]"
              :placeholder="`请输入${f.label}`"
            />
            <el-input-number
              v-else-if="f.type === 'number'"
              v-model="formData[f.prop]"
              :step="f.prop === 'value' || f.prop === 'tonnage' || f.prop === 'original_price' ? 0.1 : 0.01"
              :precision="f.prop === 'k_type' || f.prop === 'k_brand' || f.prop === 'base_coefficient' || f.prop === 'coefficient' ? 4 : 2"
              style="width: 100%"
            />
            <el-switch
              v-else-if="f.type === 'switch'"
              v-model="formData[f.prop]"
              active-text="启用"
              inactive-text="禁用"
            />
          </el-form-item>
        </el-form>
        <template #footer>
          <el-button @click="dialogVisible = false">取消</el-button>
          <el-button type="primary" :loading="submitting" @click="handleSubmit">
            {{ editingRow ? '保存' : '创建' }}
          </el-button>
        </template>
      </el-dialog>
    </div>
  </div>
</template>

<style scoped>
.config-manage {
  min-height: calc(100vh - var(--header-h, 56px) - 40px);
  background: var(--color-bg);
  padding-bottom: var(--sp-8);
}
.app-container {
  padding-top: var(--sp-6);
}
.tab-toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: var(--sp-4);
  gap: var(--sp-3);
}
.coef-tip {
  font-size: var(--fs-sm);
  color: var(--color-text-tertiary);
}
:deep(.el-tabs__content) {
  padding: var(--sp-4) var(--sp-5);
}
@media (max-width: 768px) {
  .tab-toolbar {
    flex-direction: column;
    align-items: flex-start;
    gap: var(--sp-2);
  }
  :deep(.el-tabs__content) {
    padding: var(--sp-3) var(--sp-2);
  }
}
</style>
