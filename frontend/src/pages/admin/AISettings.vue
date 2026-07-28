<template>
  <div class="ai-settings-page">
    <div class="page-header">
      <h2>AI 配置</h2>
      <el-button type="primary" :icon="Plus" @click="openCreateDialog">新建配置</el-button>
    </div>

    <!-- 功能绑定区 -->
    <div class="card">
      <div class="card-title">功能绑定</div>
      <el-table :data="bindings" border stripe style="width: 100%">
        <el-table-column label="功能" min-width="180">
          <template #default="{ row }">
            <span class="feature-label">{{ row.feature_label }}</span>
          </template>
        </el-table-column>
        <el-table-column label="绑定的配置" min-width="220">
          <template #default="{ row }">
            <el-select
              :model-value="row.config_id || 0"
              placeholder="请选择配置"
              style="width: 100%"
              @update:model-value="(val: number) => handleBind(row.feature_key, val)"
            >
              <el-option
                v-for="cfg in configs"
                :key="cfg.id"
                :value="cfg.id"
                :label="`${cfg.name}（${cfg.model}）`"
                :disabled="!cfg.is_active"
              />
            </el-select>
          </template>
        </el-table-column>
      </el-table>
    </div>

    <!-- 配置列表 -->
    <div class="card">
      <div class="card-title">配置列表</div>
      <el-table :data="configs" border stripe style="width: 100%">
        <el-table-column prop="name" label="名称" min-width="120" />
        <el-table-column prop="api_key" label="API Key" min-width="180">
          <template #default="{ row }">
            <span class="mono">{{ row.api_key || '（未设置）' }}</span>
          </template>
        </el-table-column>
        <el-table-column prop="base_url" label="Base URL" min-width="200">
          <template #default="{ row }">
            <span class="mono">{{ row.base_url || '（未设置）' }}</span>
          </template>
        </el-table-column>
        <el-table-column prop="model" label="模型" min-width="140" />
        <el-table-column label="状态" width="80">
          <template #default="{ row }">
            <el-tag :type="row.is_active ? 'success' : 'info'" size="small">
              {{ row.is_active ? '启用' : '停用' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="操作" width="100" fixed="right" align="center">
          <template #default="{ row }">
            <el-dropdown trigger="click" @command="(cmd: string) => handleAction(cmd, row)">
              <el-button type="primary" size="small">
                操作<el-icon class="el-icon--right"><ArrowDown /></el-icon>
              </el-button>
              <template #dropdown>
                <el-dropdown-menu>
                  <el-dropdown-item command="test">测试</el-dropdown-item>
                  <el-dropdown-item command="edit">编辑</el-dropdown-item>
                  <el-dropdown-item command="delete" divided>删除</el-dropdown-item>
                </el-dropdown-menu>
              </template>
            </el-dropdown>
          </template>
        </el-table-column>
      </el-table>
    </div>

    <!-- 新建/编辑对话框 -->
    <el-dialog
      v-model="dialogVisible"
      :title="dialogMode === 'create' ? '新建 AI 配置' : '编辑 AI 配置'"
      width="600px"
      destroy-on-close
    >
      <el-form
        ref="formRef"
        :model="form"
        :rules="rules"
        label-width="100px"
        autocomplete="off"
      >
        <el-form-item label="名称" prop="name">
          <el-input
            v-model="form.name"
            placeholder="如 DeepSeek 主配置、OpenAI 备用"
            clearable
            autocomplete="off"
          />
        </el-form-item>
        <el-form-item label="API Key" prop="api_key">
          <el-input
            v-model="form.api_key"
            type="password"
            show-password
            :placeholder="dialogMode === 'edit' ? '留空表示不修改' : '请输入完整 API Key'"
            clearable
            autocomplete="new-password"
          />
        </el-form-item>
        <el-form-item label="Base URL" prop="base_url">
          <el-input v-model="form.base_url" placeholder="https://api.deepseek.com" clearable autocomplete="off" />
        </el-form-item>
        <el-form-item label="模型" prop="model">
          <el-input v-model="form.model" placeholder="deepseek-v4-flash" clearable autocomplete="off" />
        </el-form-item>
        <el-form-item label="描述" prop="description">
          <el-input v-model="form.description" placeholder="可选备注" clearable autocomplete="off" />
        </el-form-item>
        <el-form-item v-if="dialogMode === 'edit'" label="启用状态">
          <el-switch v-model="form.is_active" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="saving" @click="handleSave">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { ElMessage, ElMessageBox, type FormInstance, type FormItemRule } from 'element-plus'
import { Plus, ArrowDown } from '@element-plus/icons-vue'
import { adminApi, type AIConfig, type FeatureBinding } from '@/api/admin'

const configs = ref<AIConfig[]>([])
const bindings = ref<FeatureBinding[]>([])

const dialogVisible = ref(false)
const dialogMode = ref<'create' | 'edit'>('create')
const editingId = ref<number | null>(null)
const saving = ref(false)
const testingId = ref<number | null>(null)

const formRef = ref<FormInstance>()
const form = ref({
  name: '',
  api_key: '',
  base_url: '',
  model: '',
  description: '',
  is_active: true
})

// api_key 仅在新建时必填（编辑时留空表示不修改）
const rules = computed<Record<string, FormItemRule[]>>(() => ({
  name: [{ required: true, message: '请输入名称', trigger: 'blur' }],
  api_key:
    dialogMode.value === 'create'
      ? [{ required: true, message: '请输入 API Key', trigger: 'blur' }]
      : [],
  base_url: [{ required: true, message: '请输入 Base URL', trigger: 'blur' }],
  model: [{ required: true, message: '请输入模型名', trigger: 'blur' }]
}))

async function loadAll() {
  await Promise.all([loadConfigs(), loadBindings()])
}

async function loadConfigs() {
  try {
    const res = await adminApi.listAIConfigs()
    if (res.code === 200) {
      configs.value = res.data as AIConfig[]
    }
  } catch (e) {
    console.error('加载配置列表失败:', e)
  }
}

async function loadBindings() {
  try {
    const res = await adminApi.listFeatureBindings()
    if (res.code === 200) {
      bindings.value = res.data as FeatureBinding[]
    }
  } catch (e) {
    console.error('加载功能绑定失败:', e)
  }
}

function openCreateDialog() {
  dialogMode.value = 'create'
  editingId.value = null
  form.value = {
    name: '',
    api_key: '',
    base_url: '',
    model: '',
    description: '',
    is_active: true
  }
  dialogVisible.value = true
}

function openEditDialog(row: AIConfig) {
  dialogMode.value = 'edit'
  editingId.value = row.id
  form.value = {
    name: row.name,
    api_key: '',
    base_url: row.base_url,
    model: row.model,
    description: row.description || '',
    is_active: row.is_active
  }
  dialogVisible.value = true
}

async function handleSave() {
  if (!formRef.value) return
  await formRef.value.validate(async (valid) => {
    if (!valid) return
    saving.value = true
    try {
      if (dialogMode.value === 'create') {
        const res = await adminApi.createAIConfig({
          name: form.value.name,
          api_key: form.value.api_key,
          base_url: form.value.base_url,
          model: form.value.model,
          description: form.value.description
        })
        if (res.code === 200) {
          ElMessage.success('配置已创建')
          dialogVisible.value = false
          await loadConfigs()
        } else {
          ElMessage.error(res.message || '创建失败')
        }
      } else if (editingId.value !== null) {
        const res = await adminApi.updateAIConfig(editingId.value, {
          name: form.value.name,
          api_key: form.value.api_key || undefined,
          base_url: form.value.base_url,
          model: form.value.model,
          description: form.value.description,
          is_active: form.value.is_active
        })
        if (res.code === 200) {
          ElMessage.success('配置已更新')
          dialogVisible.value = false
          await Promise.all([loadConfigs(), loadBindings()])
        } else {
          ElMessage.error(res.message || '更新失败')
        }
      }
    } catch (e) {
      console.error('保存配置失败:', e)
      ElMessage.error('保存失败')
    } finally {
      saving.value = false
    }
  })
}

async function handleDelete(row: AIConfig) {
  try {
    const res = await adminApi.deleteAIConfig(row.id)
    if (res.code === 200) {
      ElMessage.success('配置已删除')
      await Promise.all([loadConfigs(), loadBindings()])
    } else {
      ElMessage.error(res.message || '删除失败')
    }
  } catch (e) {
    console.error('删除配置失败:', e)
    ElMessage.error('删除失败')
  }
}

async function handleTest(row: AIConfig) {
  testingId.value = row.id
  try {
    const res = await adminApi.testAIConfig(row.id)
    if (res.code === 200) {
      ElMessage.success(res.message || '连接成功')
    } else {
      ElMessage.error(res.message || '连接失败')
    }
  } catch (e: unknown) {
    const err = e as { response?: { data?: { message?: string } } }
    const msg = err?.response?.data?.message || '连接失败'
    ElMessage.error(msg)
  } finally {
    testingId.value = null
  }
}

// 操作下拉菜单统一入口
async function handleAction(cmd: string, row: AIConfig) {
  switch (cmd) {
    case 'test':
      await handleTest(row)
      break
    case 'edit':
      openEditDialog(row)
      break
    case 'delete':
      try {
        await ElMessageBox.confirm('确定删除该配置？被绑定的配置无法删除。', '提示', {
          type: 'warning',
          confirmButtonText: '确定',
          cancelButtonText: '取消'
        })
        await handleDelete(row)
      } catch {
        // 用户取消
      }
      break
  }
}

async function handleBind(featureKey: string, configId: number) {
  try {
    const res = await adminApi.setFeatureBinding(featureKey, configId)
    if (res.code === 200) {
      ElMessage.success('绑定已更新')
      await loadBindings()
    } else {
      ElMessage.error(res.message || '绑定失败')
    }
  } catch (e: unknown) {
    const err = e as { response?: { data?: { message?: string } } }
    const msg = err?.response?.data?.message || '绑定失败'
    ElMessage.error(msg)
    await loadBindings()
  }
}

onMounted(() => {
  loadAll()
})
</script>

<style scoped>
.ai-settings-page {
  padding: 20px;
  max-width: 1100px;
  margin: 0 auto;
}

.page-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 20px;
}

.page-header h2 {
  font-size: 22px;
  color: #303133;
}

.card {
  background: #fff;
  border-radius: 12px;
  padding: 24px;
  box-shadow: 0 2px 12px rgba(0, 0, 0, 0.06);
  margin-bottom: 20px;
}

.card-title {
  font-size: 16px;
  color: #303133;
  font-weight: 500;
  margin-bottom: 16px;
}

.mono {
  font-family: 'Courier New', monospace;
  word-break: break-all;
  font-size: 13px;
}

.feature-label {
  font-weight: 500;
  color: #303133;
}

@media screen and (max-width: 768px) {
  .ai-settings-page {
    padding: 12px;
  }

  .card {
    padding: 16px;
  }

  .page-header {
    flex-wrap: wrap;
    gap: 8px;
  }

  .page-header h2 {
    font-size: 18px;
  }

  .el-table {
    overflow-x: auto;
  }
}
</style>
