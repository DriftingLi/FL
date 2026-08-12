<template>
  <el-dialog
    v-model="visible"
    title="使用自定义 API Key"
    width="480px"
    :close-on-click-modal="false"
    @close="handleClose"
  >
    <div class="custom-hint">
      填写您的 OpenAI 格式 API Key，仅本次会话有效，不会被保存到服务器。
      如需持久保存，请登录后添加到"我的模型"。
    </div>
    <el-form ref="formRef" :model="formData" :rules="rules" label-width="90px" class="custom-form">
      <el-form-item label="模型名" prop="model">
        <el-input v-model="formData.model" placeholder="如：gpt-4o、deepseek-chat" maxlength="100" />
      </el-form-item>
      <el-form-item label="Base URL" prop="base_url">
        <el-input v-model="formData.base_url" placeholder="如：https://api.openai.com/v1" />
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
        <el-button type="primary" @click="handleConfirm">确认使用</el-button>
        <el-button @click="handleClose">取消</el-button>
      </el-form-item>
    </el-form>
  </el-dialog>
</template>

<script setup lang="ts">
import { ref, reactive, computed, watch } from 'vue'
import { type FormInstance } from 'element-plus'
import { useAIAssistantStore } from '@/stores/aiAssistant'

const props = defineProps<{ modelValue: boolean }>()
const emit = defineEmits<{
  (e: 'update:modelValue', v: boolean): void
}>()

const store = useAIAssistantStore()

const visible = computed({
  get: () => props.modelValue,
  set: (v) => emit('update:modelValue', v)
})

const formRef = ref<FormInstance>()

const formData = reactive({
  model: '',
  base_url: '',
  api_key: ''
})

const rules = {
  model: [{ required: true, message: '请输入模型名', trigger: 'blur' }],
  base_url: [{ required: true, message: '请输入 Base URL', trigger: 'blur' }],
  api_key: [{ required: true, message: '请输入 API Key', trigger: 'blur' }]
}

watch(visible, (v) => {
  if (v) {
    formData.model = ''
    formData.base_url = ''
    formData.api_key = ''
    formRef.value?.clearValidate()
  }
})

async function handleConfirm() {
  const valid = await formRef.value?.validate().catch(() => false)
  if (!valid) return
  store.selectCustomModel(formData.api_key, formData.base_url, formData.model)
  visible.value = false
}

function handleClose() {
  visible.value = false
}
</script>

<style scoped>
.custom-hint {
  font-size: 13px;
  color: var(--color-text-tertiary, #94a3b8);
  line-height: 1.6;
  margin-bottom: 16px;
  padding: 10px 12px;
  background: var(--color-bg-page, #f8fafc);
  border-radius: 8px;
}

.custom-form {
  margin-top: 8px;
}
</style>
