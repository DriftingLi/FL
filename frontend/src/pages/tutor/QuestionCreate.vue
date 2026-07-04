<template>
  <div class="question-create">
    <h2>{{ isEdit ? '编辑题目' : '新增题目' }}</h2>
    <el-form :model="form" label-width="100px" style="max-width: 700px">
      <el-form-item label="题型" required>
        <el-select v-model="form.type" :disabled="isEdit" @change="onTypeChange">
          <el-option label="单选题" value="single_choice" />
          <el-option label="多选题" value="multi_choice" />
          <el-option label="判断题" value="true_false" />
          <el-option label="故障识图" value="fault_image" />
          <el-option label="简答题" value="short_answer" />
        </el-select>
      </el-form-item>
      <el-form-item label="等级" required>
        <el-select v-model="form.level">
          <el-option label="初级" value="beginner" />
          <el-option label="中级" value="intermediate" />
          <el-option label="高级" value="advanced" />
        </el-select>
      </el-form-item>
      <el-form-item label="知识点">
        <el-select v-model="form.knowledge_point_id" clearable placeholder="选择知识点">
          <el-option v-for="kp in knowledgePoints" :key="kp.id" :label="kp.name" :value="kp.id" />
        </el-select>
      </el-form-item>
      <el-form-item label="题干" required>
        <el-input v-model="form.content" type="textarea" :rows="3" placeholder="请输入题干" />
      </el-form-item>
      <el-form-item v-if="form.type === 'fault_image'" label="图片">
        <div class="image-upload-area">
          <el-upload
            class="image-uploader"
            :show-file-list="false"
            :before-upload="beforeImageUpload"
            :http-request="handleImageUpload"
            accept=".png,.jpg,.jpeg,.gif,.webp,.bmp"
          >
            <img v-if="form.image_url" :src="form.image_url" class="image-preview" />
            <div v-else class="upload-placeholder">
              <el-icon :size="28"><Plus /></el-icon>
              <span>点击上传图片</span>
            </div>
          </el-upload>
          <div v-if="form.image_url" class="image-actions">
            <el-button type="danger" size="small" @click="removeImage">删除图片</el-button>
          </div>
          <div class="image-upload-tip">
            支持格式：PNG、JPG、JPEG、GIF、WebP、BMP，最大5MB
          </div>
          <el-divider>或手动输入URL</el-divider>
          <el-input v-model="form.image_url" placeholder="输入图片URL地址" clearable />
        </div>
      </el-form-item>
      <el-form-item v-if="hasOptions" label="选项" required>
        <div v-for="key in optionKeys" :key="key" class="option-row">
          <span class="opt-key">{{ key }}</span>
          <el-input v-model="form.options[key]" :placeholder="`选项${key}内容`" />
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
        <el-input v-model="form.reference_answer" type="textarea" :rows="3" placeholder="请输入参考答案" />
      </el-form-item>
      <el-form-item v-if="form.type === 'short_answer'" label="评分标准">
        <el-input v-model="form.scoring_criteria" type="textarea" :rows="2" placeholder="请输入评分标准" />
      </el-form-item>
      <el-form-item label="分值">
        <el-input-number v-model="form.score" :min="1" :max="50" />
      </el-form-item>
      <el-form-item label="解析">
        <el-input v-model="form.explanation" type="textarea" :rows="2" placeholder="请输入解析" />
      </el-form-item>
      <el-form-item>
        <el-button type="primary" @click="submitForm" :loading="submitting">{{ isEdit ? '更新' : '创建' }}</el-button>
        <el-button @click="$router.back()">取消</el-button>
      </el-form-item>
    </el-form>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { Plus } from '@element-plus/icons-vue'
import { ElMessage } from 'element-plus'
import { questionBankApi } from '@/api/questionBank'

const route = useRoute()
const router = useRouter()

const isEdit = computed(() => !!route.query.id)
const hasOptions = computed(() => ['single_choice', 'multi_choice', 'fault_image'].includes(form.value.type))
const optionKeys = ['A', 'B', 'C', 'D']

const submitting = ref(false)
const knowledgePoints = ref([])
const multiAnswer = ref([])
const imageUploading = ref(false)

const form = ref({
  type: 'single_choice',
  level: 'beginner',
  content: '',
  options: { A: '', B: '', C: '', D: '' },
  answer: '',
  explanation: '',
  image_url: '',
  reference_answer: '',
  scoring_criteria: '',
  score: 3,
  knowledge_point_id: null,
  status: 'pending'
})

onMounted(async () => {
  try {
    const res = await questionBankApi.getKnowledgePoints()
    knowledgePoints.value = res.data || []
  } catch (e) {}

  if (isEdit.value) {
    try {
      const res = await questionBankApi.getQuestion(route.query.id)
      const q = res.data
      form.value = { ...form.value, ...q }
      if (q.type === 'multi_choice' && q.answer) {
        multiAnswer.value = q.answer.split(',')
      }
    } catch (e) {
      ElMessage.error('获取题目失败')
    }
  }
})

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

function beforeImageUpload(file) {
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

async function handleImageUpload(options) {
  imageUploading.value = true
  try {
    const formData = new FormData()
    formData.append('image', options.file)
    const res = await questionBankApi.uploadImage(formData)
    form.value.image_url = res.data.url
    ElMessage.success('图片上传成功')
  } catch (e) {
    ElMessage.error(e.response?.data?.message || '图片上传失败')
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
    const data = { ...form.value }
    if (data.type === 'multi_choice') {
      data.answer = multiAnswer.value.sort().join(',')
    }
    if (isEdit.value) {
      await questionBankApi.updateQuestion(route.query.id, data)
      ElMessage.success('更新成功')
    } else {
      await questionBankApi.createQuestion(data)
      ElMessage.success('创建成功')
    }
    router.push('/training/tutor/question-manage')
  } catch (e) {
    ElMessage.error(e.message || '操作失败')
  } finally {
    submitting.value = false
  }
}
</script>

<style scoped>
.question-create h2 { margin-bottom: 20px; }
.option-row { display: flex; align-items: center; gap: 10px; margin-bottom: 8px; }
.opt-key { width: 24px; font-weight: bold; text-align: center; }
.option-row .el-input { flex: 1; }

.image-upload-area { width: 100%; }
.image-uploader :deep(.el-upload) {
  border: 1px dashed #dcdfe6;
  border-radius: 8px;
  cursor: pointer;
  overflow: hidden;
  transition: border-color 0.3s;
}
.image-uploader :deep(.el-upload:hover) {
  border-color: #409eff;
}
.image-preview {
  width: 300px;
  max-height: 200px;
  object-fit: contain;
  display: block;
}
.upload-placeholder {
  width: 300px;
  height: 160px;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 8px;
  color: #909399;
  font-size: 13px;
}
.image-actions {
  margin-top: 8px;
}
.image-upload-tip {
  color: #909399;
  font-size: 12px;
  margin-top: 6px;
}
</style>
