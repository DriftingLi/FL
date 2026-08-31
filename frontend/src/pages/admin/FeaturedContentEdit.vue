<template>
  <div class="featured-edit-page">
    <div class="page-header">
      <h2>{{ isEdit ? '编辑内容' : '新建内容' }}</h2>
      <el-button @click="goBack">返回列表</el-button>
    </div>

    <div class="edit-card" v-loading="loading">
      <el-form ref="formRef" :model="form" :rules="rules" label-width="100px">
        <el-form-item label="标题" prop="title">
          <el-input
            v-model="form.title"
            placeholder="请输入文章标题"
            maxlength="200"
            show-word-limit
          />
        </el-form-item>

        <el-form-item label="分类" prop="category">
          <el-select v-model="form.category" placeholder="请选择分类" style="width: 100%">
            <el-option
              v-for="opt in featuredCategoryOptions"
              :key="opt.value"
              :label="opt.label"
              :value="opt.value"
            />
          </el-select>
        </el-form-item>

        <el-form-item label="来源">
          <el-input
            v-model="form.source"
            placeholder="请输入来源（如：和润天下、行业媒体等）"
            maxlength="100"
          />
        </el-form-item>

        <el-form-item label="摘要">
          <el-input
            v-model="form.summary"
            type="textarea"
            :rows="3"
            placeholder="请输入文章摘要（最多500字，将展示在首页卡片）"
            maxlength="500"
            show-word-limit
          />
        </el-form-item>

        <el-form-item label="封面图">
          <el-upload
            class="cover-uploader"
            :show-file-list="false"
            :http-request="handleCoverUpload"
            accept="image/*"
          >
            <img v-if="form.cover_image" :src="resolveFileUrl(form.cover_image)" class="cover-preview" alt="封面预览" />
            <div v-else class="cover-placeholder">
              <el-icon><Plus /></el-icon>
              <span>点击上传封面</span>
            </div>
          </el-upload>
          <div class="cover-actions" v-if="form.cover_image">
            <el-button link type="danger" @click="form.cover_image = ''">移除封面</el-button>
          </div>
        </el-form-item>

        <el-form-item label="正文" prop="content">
          <MarkdownEditor
            ref="markdownEditorRef"
            v-model="form.content"
            :height="560"
            :upload-url="uploadUrl"
            placeholder="请输入正文内容（支持 Markdown 语法，可粘贴或上传图片）..."
          />
        </el-form-item>

        <el-form-item label="排序">
          <el-input-number v-model="form.sort_order" :min="0" :max="9999" />
        </el-form-item>

        <el-form-item label="状态">
          <el-radio-group v-model="form.status">
            <el-radio :value="0">保存草稿</el-radio>
            <el-radio :value="1">立即发布</el-radio>
          </el-radio-group>
        </el-form-item>

        <el-form-item>
          <el-button type="primary" :loading="saving" @click="handleSave">
            {{ form.status === 1 ? '保存并发布' : '保存草稿' }}
          </el-button>
          <el-button @click="goBack">取消</el-button>
        </el-form-item>
      </el-form>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { Plus } from '@element-plus/icons-vue'
import { ElMessage } from 'element-plus'
import type { FormInstance, FormRules } from 'element-plus'
import MarkdownEditor from '@/components/tutor/MarkdownEditor.vue'
import { adminFeaturedApi, featuredCategoryOptions } from '@/api/featured'
import { resolveFileUrl } from '@/utils/fileUrl'

const route = useRoute()
const router = useRouter()

const formRef = ref<FormInstance | null>(null)
const markdownEditorRef = ref<{ getValue: () => string } | null>(null)
const loading = ref(false)
const saving = ref(false)

const isEdit = computed(() => !!route.params.id)
const editId = computed(() => Number(route.params.id))

// Vditor 不走 axios，需要带 /api 前缀的完整相对路径
const uploadUrl = '/api/admin/featured-content/upload-image'

const form = reactive({
  title: '',
  category: '',
  source: '',
  summary: '',
  cover_image: '',
  content: '',
  sort_order: 0,
  status: 0 as number
})

const rules: FormRules = {
  title: [
    { required: true, message: '请输入文章标题', trigger: 'blur' },
    { min: 2, max: 200, message: '标题长度为 2-200 个字符', trigger: 'blur' }
  ],
  category: [
    { required: true, message: '请选择分类', trigger: 'change' }
  ]
}

async function loadDetail() {
  if (!isEdit.value) return
  loading.value = true
  try {
    const d = await adminFeaturedApi.getDetail(editId.value)
    form.title = d.title || ''
    form.category = d.category || ''
    form.source = d.source || ''
    form.summary = d.summary || ''
    form.cover_image = d.cover_image || ''
    form.content = d.content || ''
    form.sort_order = d.sort_order || 0
    form.status = d.status ?? 0
  } catch (e: any) {
    // 错误已由全局拦截器提示
    router.push({ name: 'AdminFeaturedContentList' })
  } finally {
    loading.value = false
  }
}

async function handleCoverUpload(option: any) {
  const file = option.file as File
  if (!file) return
  try {
    const res = await adminFeaturedApi.uploadImage(file)
    // res 是后端原始 Vditor 格式：{ msg, code, data: { errFiles, succMap } }
    if (res.code === 0) {
      const succMap = res.data?.succMap || {}
      const firstUrl = Object.values(succMap)[0]
      if (firstUrl) {
        form.cover_image = firstUrl
        ElMessage.success('封面图上传成功')
      } else {
        ElMessage.error('上传失败：未返回图片 URL')
      }
    } else {
      ElMessage.error(res.msg || '封面图上传失败')
    }
  } catch (e: any) {
    ElMessage.error(e.message || '封面图上传失败')
  }
}

async function handleSave() {
  if (!formRef.value) return
  try {
    await formRef.value.validate()
  } catch {
    return
  }

  // 主动从 MarkdownEditor 获取最新正文（避免 v-model 在 ir 模式下偶发未同步）
  if (markdownEditorRef.value) {
    const latest = markdownEditorRef.value.getValue()
    if (latest) form.content = latest
  }

  if (!form.content.trim()) {
    ElMessage.warning('正文内容不能为空')
    return
  }

  saving.value = true
  const payload = {
    title: form.title.trim(),
    category: form.category,
    source: form.source,
    summary: form.summary,
    cover_image: form.cover_image,
    content: form.content,
    sort_order: form.sort_order,
    status: form.status
  }
  try {
    if (isEdit.value) {
      await adminFeaturedApi.update(editId.value, payload)
      ElMessage.success(form.status === 1 ? '已保存并发布' : '草稿已保存')
      router.push({ name: 'AdminFeaturedContentList' })
    } else {
      await adminFeaturedApi.create(payload)
      ElMessage.success(form.status === 1 ? '已创建并发布' : '草稿已创建')
      router.push({ name: 'AdminFeaturedContentList' })
    }
  } catch (e: any) {
    // 错误已由全局拦截器提示
  } finally {
    saving.value = false
  }
}

function goBack() {
  router.push({ name: 'AdminFeaturedContentList' })
}

onMounted(() => {
  loadDetail()
})
</script>

<style scoped>
.featured-edit-page {
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
  margin: 0;
  font-size: 22px;
  font-weight: 600;
  color: var(--color-text-primary);
}

.edit-card {
  background: var(--color-text-inverse);
  border-radius: 12px;
  padding: 32px 40px;
  box-shadow: 0 2px 12px rgba(0, 0, 0, 0.06);
}

.cover-uploader {
  display: inline-block;
  width: 280px;
  height: 158px;
  border: 1px dashed var(--color-border-dark);
  border-radius: 8px;
  overflow: hidden;
  cursor: pointer;
  transition: border-color var(--duration-base) var(--ease-default);
}

.cover-uploader:hover {
  border-color: var(--el-color-primary, var(--color-primary-500));
}

.cover-preview {
  width: 100%;
  height: 100%;
  object-fit: cover;
  display: block;
}

.cover-placeholder {
  width: 100%;
  height: 100%;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 8px;
  color: var(--color-text-muted);
  font-size: 13px;
  background: var(--color-bg-page);
}

.cover-placeholder .el-icon {
  font-size: 28px;
}

.cover-actions {
  margin-top: 8px;
}

@media (max-width: 768px) {
  .edit-card {
    padding: 20px 16px;
  }

  .cover-uploader {
    width: 100%;
  }
}
</style>
