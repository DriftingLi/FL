<template>
  <div>
    <UiPageHeader
      :title="isEdit ? '编辑题目' : '新增题目'"
      :subtitle="isEdit ? '修改后需重新提交审核' : '创建后进入待审核状态'"
      back
      @back="router.back()"
    />

    <!-- 编辑态下题目本体加载失败必须阻断渲染：
         否则会以空表单呈现，保存时把原题内容整体覆盖掉 -->
    <UiErrorState
      v-if="pageError"
      title="题目加载失败"
      description="未能读取该题目内容，重试成功前不会渲染表单，避免误覆盖原题。"
      :retrying="retrying"
      @retry="handleRetry"
    />

    <UiSkeleton v-else-if="pageLoading" variant="text" :rows="10" />

    <UiCard v-else padding="lg">
      <el-form :model="form" label-width="100px" class="max-w-[700px]">
        <el-form-item label="题型" required>
          <UiSelect
            v-model="form.type"
            :options="TYPE_OPTIONS"
            :disabled="isEdit"
            @change="onTypeChange"
          />
        </el-form-item>

        <el-form-item label="所属证件" required>
          <el-select v-model="form.credential_id" placeholder="必选" class="w-full">
            <el-option-group label="特种作业">
              <el-option
                v-for="c in credentials.filter((x) => x.category === 'special_operation')"
                :key="c.id"
                :label="c.name"
                :value="c.id"
              />
            </el-option-group>
            <el-option-group label="技能等级">
              <el-option
                v-for="c in credentials.filter((x) => x.category === 'skill_level')"
                :key="c.id"
                :label="c.name"
                :value="c.id"
              />
            </el-option-group>
          </el-select>
        </el-form-item>

        <el-form-item label="考点标签">
          <el-select
            v-model="form.tag_ids"
            multiple
            filterable
            collapse-tags
            placeholder="选择标签（可多选）"
            class="w-full"
          >
            <el-option v-for="t in tags" :key="t.id" :label="t.name" :value="t.id" />
          </el-select>
        </el-form-item>

        <el-form-item label="题干" required>
          <UiInput v-model="form.content" type="textarea" :rows="3" placeholder="请输入题干" />
        </el-form-item>

        <el-form-item v-if="form.type === 'fault_image'" label="图片">
          <div v-loading="imageUploading" class="w-full">
            <el-upload
              class="[&_.el-upload]:flex [&_.el-upload]:cursor-pointer [&_.el-upload]:overflow-hidden [&_.el-upload]:rounded-card [&_.el-upload]:border [&_.el-upload]:border-dashed [&_.el-upload]:border-line-strong [&_.el-upload]:transition-colors [&_.el-upload:hover]:border-ui-500"
              :show-file-list="false"
              :before-upload="beforeImageUpload"
              :http-request="handleImageUpload"
              accept=".png,.jpg,.jpeg,.gif,.webp,.bmp"
            >
              <img v-if="form.image_url" :src="form.image_url" class="block w-[300px] max-h-[200px] object-contain" />
              <div v-else class="flex h-40 w-[300px] flex-col items-center justify-center gap-2 text-[13px] text-ink-3">
                <el-icon :size="28"><Plus /></el-icon>
                <span>点击上传图片</span>
              </div>
            </el-upload>

            <div v-if="form.image_url" class="mt-2">
              <UiButton variant="danger" size="small" @click="removeImage">删除图片</UiButton>
            </div>

            <p class="mt-1.5 text-xs text-ink-3">支持格式：PNG、JPG、JPEG、GIF、WebP、BMP，最大5MB</p>

            <el-divider>或手动输入URL</el-divider>
            <UiInput v-model="form.image_url" placeholder="输入图片URL地址" clearable />
          </div>
        </el-form-item>

        <el-form-item v-if="hasOptions" label="选项" required>
          <div v-for="key in optionKeys" :key="key" class="mb-2 flex items-center gap-2.5 [&_.el-input]:flex-1">
            <span class="w-6 text-center font-bold">{{ key }}</span>
            <UiInput v-model="form.options![key]" :placeholder="`选项${key}内容`" />
          </div>
        </el-form-item>

        <el-form-item v-if="form.type === 'true_false'" label="正确答案" required>
          <el-radio-group v-model="form.answer">
            <el-radio value="对">对</el-radio>
            <el-radio value="错">错</el-radio>
          </el-radio-group>
        </el-form-item>

        <el-form-item v-else-if="form.type === 'single_choice' || form.type === 'fault_image'" label="正确答案" required>
          <el-radio-group v-model="form.answer">
            <el-radio v-for="key in optionKeys" :key="key" :value="key">{{ key }}</el-radio>
          </el-radio-group>
        </el-form-item>

        <el-form-item v-else-if="form.type === 'multi_choice'" label="正确答案" required>
          <el-checkbox-group v-model="multiAnswer">
            <el-checkbox v-for="key in optionKeys" :key="key" :value="key" :label="key">{{ key }}</el-checkbox>
          </el-checkbox-group>
        </el-form-item>

        <el-form-item v-if="form.type === 'short_answer'" label="参考答案">
          <UiInput v-model="form.reference_answer" type="textarea" :rows="3" placeholder="请输入参考答案" />
        </el-form-item>

        <el-form-item v-if="form.type === 'short_answer'" label="评分标准">
          <UiInput v-model="form.scoring_criteria" type="textarea" :rows="2" placeholder="请输入评分标准" />
        </el-form-item>

        <el-form-item label="分值">
          <el-input-number v-model="form.score" :min="1" :max="50" />
        </el-form-item>

        <el-form-item label="解析">
          <UiInput v-model="form.explanation" type="textarea" :rows="2" placeholder="请输入解析" />
        </el-form-item>

        <el-form-item>
          <div class="flex gap-3">
            <UiButton variant="primary" :loading="submitting" @click="submitForm">
              {{ isEdit ? '更新' : '创建' }}
            </UiButton>
            <UiButton @click="router.back()">取消</UiButton>
          </div>
        </el-form-item>
      </el-form>
    </UiCard>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { Plus } from '@element-plus/icons-vue'
import { ElMessage } from 'element-plus'
import { questionBankApi, type QuestionPayload } from '@/api/questionBank'
import { credentialApi, type CredentialDict } from '@/api/credential'
import { trainingApi } from '@/api/training'
import UiPageHeader from '@/components/ui/UiPageHeader.vue'
import UiCard from '@/components/ui/UiCard.vue'
import UiButton from '@/components/ui/UiButton.vue'
import UiInput from '@/components/ui/UiInput.vue'
import UiSelect, { type UiSelectOption } from '@/components/ui/UiSelect.vue'
import UiSkeleton from '@/components/ui/UiSkeleton.vue'
import UiErrorState from '@/components/ui/UiErrorState.vue'
import { useAsyncPage } from '@/composables/useAsyncPage'

const route = useRoute()
const router = useRouter()

const isEdit = computed(() => !!route.query.id)
const hasOptions = computed(() =>
  ['single_choice', 'multi_choice', 'fault_image'].includes(form.value.type)
)
const optionKeys = ['A', 'B', 'C', 'D'] as const

const TYPE_OPTIONS: UiSelectOption[] = [
  { label: '单选题', value: 'single_choice' },
  { label: '多选题', value: 'multi_choice' },
  { label: '判断题', value: 'true_false' },
  { label: '故障识图', value: 'fault_image' },
  { label: '简答题', value: 'short_answer' }
]

const submitting = ref(false)
const tags = ref<{ id: number; name: string }[]>([])
const credentials = ref<CredentialDict[]>([])
const multiAnswer = ref<string[]>([])
const imageUploading = ref(false)

// 三态收编 useAsyncPage（#401）：错误详情由拦截器统一 toast；编辑态题目加载失败必须阻断渲染，
// 避免以空表单呈现、保存时把原题内容整体覆盖掉
const {
  loading: pageLoading,
  loadError: pageError,
  retrying,
  retry: handleRetry,
  run: loadPage
} = useAsyncPage(async () => {
  await loadDicts()
  if (isEdit.value) await loadQuestion()
})

const form = ref<{
  type: string
  content: string
  options: { A: string; B: string; C: string; D: string } | null
  answer: string
  explanation: string
  image_url: string
  reference_answer: string
  scoring_criteria: string
  score: number
  tag_ids: number[]
  credential_id: number | null
  status: string
}>({
  type: 'single_choice',
  content: '',
  options: { A: '', B: '', C: '', D: '' },
  answer: '',
  explanation: '',
  image_url: '',
  reference_answer: '',
  scoring_criteria: '',
  score: 3,
  tag_ids: [],
  credential_id: null,
  status: 'pending'
})

async function loadDicts() {
  // 字典拉取失败不阻断：证件/标签缺失只影响可选项，表单仍可用
  try {
    const [tagData, credData] = await Promise.all([
      trainingApi.getTags(),
      credentialApi.listCredentials()
    ])
    tags.value = tagData.tags || []
    credentials.value = credData.credentials || []
  } catch (e) {
    console.error('Failed to load dicts:', e)
  }
}

async function loadQuestion() {
  const res = await questionBankApi.getQuestion(Number(route.query.id))
  form.value = {
    ...form.value,
    ...res,
    options:
      (res.options as { A: string; B: string; C: string; D: string } | undefined) ??
      form.value.options,
    tag_ids:
      ((res as unknown as Record<string, unknown>).tag_ids as number[] | undefined) ??
      form.value.tag_ids
  }
  if (res.type === 'multi_choice' && res.answer) {
    multiAnswer.value = res.answer.split(',')
  }
}

function onTypeChange() {
  if (form.value.type === 'true_false') {
    form.value.options = null
    form.value.answer = ''
  } else if (form.value.type === 'short_answer') {
    form.value.options = null
    form.value.answer = ''
  } else {
    form.value.options = { A: '', B: '', C: '', D: '' }
    form.value.answer = ''
  }
  multiAnswer.value = []
}

function beforeImageUpload(file: File) {
  const allowedTypes = ['image/png', 'image/jpeg', 'image/gif', 'image/webp', 'image/bmp']
  if (!allowedTypes.includes(file.type)) {
    ElMessage.error('不支持的图片格式，请上传 PNG/JPG/GIF/WebP/BMP 格式')
    return false
  }
  const maxSize = 5 * 1024 * 1024
  if (file.size > maxSize) {
    ElMessage.error('图片大小不能超过5MB')
    return false
  }
  return true
}

async function handleImageUpload(options: { file: File }) {
  imageUploading.value = true
  try {
    const formData = new FormData()
    formData.append('image', options.file)
    const res = await questionBankApi.uploadImage(formData)
    form.value.image_url = res.url
    ElMessage.success('图片上传成功')
  } catch {
    /* 错误已由拦截器提示 */
  } finally {
    imageUploading.value = false
  }
}

function removeImage() {
  form.value.image_url = ''
}

async function submitForm() {
  submitting.value = true
  try {
    const data: QuestionPayload = {
      ...form.value,
      options: form.value.options ?? undefined
    }
    if (data.type === 'multi_choice') {
      data.answer = multiAnswer.value.sort().join(',')
    }
    if (isEdit.value) {
      await questionBankApi.updateQuestion(Number(route.query.id), data)
      ElMessage.success('更新成功')
    } else {
      await questionBankApi.createQuestion(data)
      ElMessage.success('创建成功')
    }
    router.push({ name: 'TutorQuestionManage' })
  } catch {
    /* 错误已由拦截器提示 */
  } finally {
    submitting.value = false
  }
}

onMounted(() => {
  loadPage()
})
</script>
