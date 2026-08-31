<template>
  <div class="ai-settings-page">
    <div class="page-header">
      <h2>AI 配置</h2>
      <UiButton variant="primary" :icon="Plus" @click="openCreateDialog">新建配置</UiButton>
    </div>

    <!-- AI助手双模式绑定（单绑×2，与其它功能分开） -->
    <UiCard padding="lg" class="card">
      <div class="card-title">AI助手模式绑定</div>
      <div class="dual-bind-grid">
        <div class="dual-bind-item">
          <div class="dual-label">普通模式</div>
          <el-select
            :model-value="normalBinding?.config_id || 0"
            placeholder="请选择配置"
            style="width: 100%"
            @update:model-value="(val: number) => handleAssistantBind('ai_assistant_normal', val)"
          >
            <el-option :value="0" label="未绑定" />
            <el-option
              v-for="cfg in configs"
              :key="cfg.id"
              :value="cfg.id"
              :label="`${cfg.name}（${cfg.model}）`"
              :disabled="!cfg.is_active"
            />
          </el-select>
        </div>
        <div class="dual-bind-item">
          <div class="dual-label">专家模式</div>
          <el-select
            :model-value="expertBinding?.config_id || 0"
            placeholder="请选择配置"
            style="width: 100%"
            @update:model-value="(val: number) => handleAssistantBind('ai_assistant_expert', val)"
          >
            <el-option :value="0" label="未绑定" />
            <el-option
              v-for="cfg in configs"
              :key="cfg.id"
              :value="cfg.id"
              :label="`${cfg.name}（${cfg.model}）`"
              :disabled="!cfg.is_active"
            />
          </el-select>
        </div>
      </div>
      <div class="dual-hint">普通/专家分别单绑定，不向用户暴露模型名；未绑定时该模式不可用</div>
    </UiCard>

    <!-- 其他功能绑定 -->
    <UiCard padding="lg" class="card">
      <div class="card-title">其他功能绑定</div>
      <el-table :data="otherBindings" border stripe style="width: 100%">
        <el-table-column label="功能" min-width="160">
          <template #default="{ row }">
            <div class="feature-cell">
              <span class="feature-label">{{ row.feature_label }}</span>
              <el-tag type="info" size="small">单绑定</el-tag>
            </div>
          </template>
        </el-table-column>
        <el-table-column label="绑定的配置" min-width="380">
          <template #default="{ row }">
            <el-select
              :model-value="row.config_id || 0"
              placeholder="请选择配置"
              style="width: 100%"
              @update:model-value="(val: number) => handleBind(row.feature_key, val)"
            >
              <el-option :value="0" label="未绑定" />
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
    </UiCard>

    <!-- 配置列表 -->
    <UiCard padding="lg" class="card">
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
              <UiButton variant="primary" size="small">
                操作<el-icon class="el-icon--right"><ArrowDown /></el-icon>
              </UiButton>
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
    </UiCard>

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
        <UiButton @click="dialogVisible = false">取消</UiButton>
        <UiButton variant="primary" :loading="saving" @click="handleSave">保存</UiButton>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { ElMessage, ElMessageBox, type FormInstance, type FormItemRule } from 'element-plus'
import { Plus, ArrowDown } from '@element-plus/icons-vue'
import { adminApi, type AIConfig, type FeatureBinding } from '@/api/admin'
import UiButton from '@/components/ui/UiButton.vue'
import UiCard from '@/components/ui/UiCard.vue'

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
    const data = await adminApi.listAIConfigs()
    if (data) {
      configs.value = data
    }
  } catch (e) {
    console.error('加载配置列表失败:', e)
  }
}

async function loadBindings() {
  try {
    const data = await adminApi.listFeatureBindings()
    if (data) bindings.value = data
  } catch (e) {
    console.error('加载功能绑定失败:', e)
  }
}

const normalBinding = computed(() => bindings.value.find(b => b.feature_key === 'ai_assistant_normal'))
const expertBinding = computed(() => bindings.value.find(b => b.feature_key === 'ai_assistant_expert'))
const otherBindings = computed(() => bindings.value.filter(b => !['ai_assistant_normal', 'ai_assistant_expert', 'ai_assistant'].includes(b.feature_key)))

async function handleAssistantBind(featureKey: string, configId: number) {
  try {
    await adminApi.setFeatureBinding(featureKey, configId)
    ElMessage.success('绑定已更新')
    await loadBindings()
  } catch {
    await loadBindings()
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
        await adminApi.createAIConfig({
          name: form.value.name,
          api_key: form.value.api_key,
          base_url: form.value.base_url,
          model: form.value.model,
          description: form.value.description
        })
        ElMessage.success('配置已创建')
        dialogVisible.value = false
        await loadConfigs()
      } else if (editingId.value !== null) {
        await adminApi.updateAIConfig(editingId.value, {
          name: form.value.name,
          api_key: form.value.api_key || undefined,
          base_url: form.value.base_url,
          model: form.value.model,
          description: form.value.description,
          is_active: form.value.is_active
        })
        ElMessage.success('配置已更新')
        dialogVisible.value = false
        await Promise.all([loadConfigs(), loadBindings()])
      }
    } catch (e) {
      console.error('保存配置失败:', e)
      /* 错误已由拦截器提示 */
    } finally {
      saving.value = false
    }
  })
}

async function handleDelete(row: AIConfig) {
  try {
    await adminApi.deleteAIConfig(row.id)
    ElMessage.success('配置已删除')
    await Promise.all([loadConfigs(), loadBindings()])
  } catch (e) {
    console.error('删除配置失败:', e)
    /* 错误已由拦截器提示 */
  }
}

async function handleTest(row: AIConfig) {
  testingId.value = row.id
  try {
    const data = await adminApi.testAIConfig(row.id)
    ElMessage.success(data?.message || '连接成功')
  } catch {
    // 拦截器已统一 toast 业务失败信息
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
    await adminApi.setFeatureBinding(featureKey, configId)
    ElMessage.success('绑定已更新')
    await loadBindings()
  } catch {
    // 拦截器已统一 toast 业务失败信息
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
  color: var(--color-text-primary);
}

.card {
  /* 容器（底色/圆角/内距/投影）已由 UiCard 承担，此处只留外边距 */
  margin-bottom: 20px;
}

.card-title {
  font-size: 16px;
  color: var(--color-text-primary);
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
  color: var(--color-text-primary);
}

.feature-cell {
  display: flex;
  align-items: center;
  gap: 8px;
}

.dual-bind-grid {
  display: grid;
  grid-template-columns: repeat(2, 1fr);
  gap: 16px;
}

.dual-bind-item {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.dual-label {
  font-size: 13px;
  font-weight: 500;
  color: var(--color-text-primary);
}

.dual-hint {
  margin-top: 12px;
  font-size: 12px;
  color: var(--color-text-tertiary);
  line-height: 1.5;
}

@media screen and (max-width: 768px) {
  .ai-settings-page {
    padding: 12px;
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

  .dual-bind-grid {
    grid-template-columns: 1fr;
  }
}
</style>
