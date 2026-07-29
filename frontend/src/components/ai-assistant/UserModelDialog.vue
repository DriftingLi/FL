<template>
  <el-dialog
    v-model="visible"
    title="管理自定义模型"
    width="600px"
    :close-on-click-modal="false"
    @close="handleClose"
  >
    <!-- 已有模型列表 -->
    <div class="user-model-list">
      <div v-if="userModels.length === 0" class="empty-hint">
        暂无自定义模型，点击下方按钮添加
      </div>
      <div v-for="m in userModels" :key="m.id" class="user-model-card">
        <div class="model-info">
          <span class="model-name">{{ m.name }}</span>
          <span class="model-meta">{{ m.model }} · {{ m.base_url }}</span>
          <span class="model-key">Key: {{ m.api_key }}</span>
        </div>
        <div class="model-actions">
          <el-button text size="small" @click="editModel(m)">编辑</el-button>
          <el-button text size="small" type="danger" @click="removeModel(m.id)">删除</el-button>
        </div>
      </div>
    </div>

    <el-divider content-position="center">
      <span class="divider-text">{{ editingId ? '编辑模型' : '新增模型' }}</span>
    </el-divider>

    <!-- 编辑/新增表单 -->
    <el-form ref="formRef" :model="formData" :rules="rules" label-width="100px" class="model-form">
      <el-form-item label="名称" prop="name">
        <el-input v-model="formData.name" placeholder="如：我的 DeepSeek" maxlength="50" />
      </el-form-item>
      <el-form-item label="模型名" prop="model">
        <el-input v-model="formData.model" placeholder="如：deepseek-chat" maxlength="100" />
      </el-form-item>
      <el-form-item label="Base URL" prop="base_url">
        <el-input v-model="formData.base_url" placeholder="如：https://api.deepseek.com" />
      </el-form-item>
      <el-form-item label="API Key" prop="api_key">
        <el-input
          v-model="formData.api_key"
          type="password"
          show-password
          placeholder="sk-..."
        />
      </el-form-item>
      <el-form-item>
        <el-button type="primary" :loading="saving" @click="handleSave">
          {{ editingId ? '更新' : '新增' }}
        </el-button>
        <el-button v-if="editingId" @click="resetForm">取消</el-button>
      </el-form-item>
    </el-form>
  </el-dialog>
</template>

<script setup lang="ts">
import { ref, reactive, watch, computed } from 'vue'
import { ElMessage, ElMessageBox, type FormInstance } from 'element-plus'
import { useAIAssistantStore } from '@/stores/aiAssistant'
import type { UserModelDTO, SaveUserModelReq } from '@/api/aiAssistant'

const props = defineProps<{ modelValue: boolean }>()
const emit = defineEmits<{
  (e: 'update:modelValue', v: boolean): void
}>()

const store = useAIAssistantStore()

const visible = computed({
  get: () => props.modelValue,
  set: (v) => emit('update:modelValue', v)
})

const userModels = computed(() => store.userModels)
const formRef = ref<FormInstance>()
const saving = ref(false)
const editingId = ref<number | null>(null)

const formData = reactive<SaveUserModelReq>({
  name: '',
  model: '',
  base_url: '',
  api_key: ''
})

const rules = {
  name: [{ required: true, message: '请输入名称', trigger: 'blur' }],
  model: [{ required: true, message: '请输入模型名', trigger: 'blur' }],
  base_url: [{ required: true, message: '请输入 Base URL', trigger: 'blur' }],
  api_key: [{ required: true, message: '请输入 API Key', trigger: 'blur' }]
}

watch(visible, (v) => {
  if (v) {
    store.loadUserModels()
    resetForm()
  }
})

function editModel(m: UserModelDTO) {
  editingId.value = m.id
  formData.name = m.name
  formData.model = m.model
  formData.base_url = m.base_url
  formData.api_key = '' // 不回显真实 key，需要重新输入
}

function resetForm() {
  editingId.value = null
  formData.name = ''
  formData.model = ''
  formData.base_url = ''
  formData.api_key = ''
  formRef.value?.clearValidate()
}

async function handleSave() {
  const valid = await formRef.value?.validate().catch(() => false)
  if (!valid) return

  saving.value = true
  try {
    const data: SaveUserModelReq = {
      id: editingId.value ?? undefined,
      name: formData.name,
      model: formData.model,
      base_url: formData.base_url,
      api_key: formData.api_key
    }
    await store.saveUserModel(data)
    ElMessage.success(editingId.value ? '更新成功' : '添加成功')
    resetForm()
  } catch (e: any) {
    // 错误已由拦截器提示
  } finally {
    saving.value = false
  }
}

async function removeModel(id: number) {
  try {
    await ElMessageBox.confirm('确定删除该模型配置？', '确认', { type: 'warning' })
  } catch {
    return
  }
  try {
    await store.deleteUserModel(id)
    ElMessage.success('删除成功')
  } catch (e: any) {
    // 错误已由拦截器提示
  }
}

function handleClose() {
  resetForm()
}
</script>

<style scoped>
.user-model-list {
  display: flex;
  flex-direction: column;
  gap: 8px;
  max-height: 280px;
  overflow-y: auto;
}

.empty-hint {
  text-align: center;
  color: var(--color-text-tertiary, #94a3b8);
  padding: 24px 0;
  font-size: 13px;
}

.user-model-card {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 10px 12px;
  background: var(--color-bg-page, #f8fafc);
  border: 1px solid var(--color-border-light, #e2e8f0);
  border-radius: 8px;
  transition: border-color 0.15s ease;
}

.user-model-card:hover {
  border-color: var(--color-brand-400, #38bdf8);
}

.model-info {
  display: flex;
  flex-direction: column;
  gap: 2px;
  flex: 1;
  min-width: 0;
}

.model-name {
  font-size: 13px;
  font-weight: 600;
  color: var(--color-text-primary, #0f172a);
}

.model-meta {
  font-size: 11px;
  color: var(--color-text-tertiary, #94a3b8);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.model-key {
  font-size: 11px;
  color: var(--color-text-muted, #cbd5e1);
  font-family: var(--font-mono, monospace);
}

.model-actions {
  display: flex;
  gap: 4px;
  flex-shrink: 0;
}

.divider-text {
  font-size: 13px;
  color: var(--color-text-secondary, #475569);
}

.model-form {
  margin-top: 8px;
}
</style>
