<template>
  <div class="featured-edit-page">
    <div class="page-header">
      <h2>{{ isEdit ? '编辑内容' : '新建内容' }}</h2>
      <UiButton @click="goBack">返回列表</UiButton>
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
          <UiUpload
            class="cover-uploader"
            :show-file-list="false"
            :http-request="handleCoverUpload"
            accept="image/*"
          >
            <img v-if="form.cover_image" :src="resolveFileUrl(form.cover_image)" class="cover-preview" alt="封面预览" />
            <div v-else class="cover-placeholder">
              <UiIcon name="plus" />
              <span>点击上传封面</span>
            </div>
          </UiUpload>
          <div class="cover-actions" v-if="form.cover_image">
            <UiButton variant="danger" link @click="form.cover_image = ''">移除封面</UiButton>
          </div>
        </el-form-item>

        <el-form-item label="正文" prop="content">
          <div class="w-full">
            <!--
              三端交集子集提示（ADR-0046 / #902）：门户与移动端都不渲染表格与公式，
              作者在这里被明确告知，而不是发布后才发现读者看到的是一堆竖线。
              **只提示不阻断**：判据留在编辑器与文档，不做后端强校验。
            -->
            <UiAlert
              v-if="outsideSubsetLabels.length"
              class="mb-2"
              type="warning"
              :closable="false"
              show-icon
              :title="outsideNotice"
              description="保存与发布不受影响；发布端预览里能看到读者实际看到的样子。"
            />
            <MarkdownEditor
              ref="markdownEditorRef"
              v-model="form.content"
              :height="560"
              :upload-url="uploadUrl"
              placeholder="请输入正文内容（支持 Markdown 语法，可直接粘贴图片）..."
            />
            <div class="mt-2 flex justify-end">
              <UiButton size="small" @click="openPreview">发布端预览</UiButton>
            </div>
          </div>
        </el-form-item>

        <el-form-item label="排序">
          <el-input-number v-model="form.sort_order" :min="0" :max="9999" />
        </el-form-item>

        <el-form-item label="状态">
          <UiRadioGroup v-model="form.status">
            <el-radio :value="0">保存草稿</el-radio>
            <el-radio :value="1">发布</el-radio>
          </UiRadioGroup>
        </el-form-item>

        <!--
          发布时间（#1748）：只在「发布」下出现。三种填法就是三种行为 ——
          留空 = 立即发布、过去 = 补录、未来 = 定时；提示文案由 describePublishTime 单点提供，
          不在模板里再写一遍规则（否则文案与提交逻辑会各漂一半）。
        -->
        <el-form-item v-if="form.status === 1" label="发布时间">
          <!-- el-form-item__content 是 flex 横排：要让提示落在框下方，必须自己起一列 -->
          <div class="publish-time-group">
            <div class="publish-time">
              <el-date-picker
                v-model="form.published_at"
                type="datetime"
                placeholder="留空 = 保存后立即发布"
                format="YYYY-MM-DD HH:mm"
                clearable
              />
              <UiButton size="small" @click="useNow">现在</UiButton>
            </div>
            <div class="publish-hint">{{ publishHint }}</div>
          </div>
        </el-form-item>

        <el-form-item>
          <UiButton variant="primary" :loading="saving" @click="handleSave">
            {{ saveLabel }}
          </UiButton>
          <UiButton @click="goBack">取消</UiButton>
        </el-form-item>
      </el-form>
    </div>

    <!-- 发布端预览（#903）：交集档渲染 + 越界语法说明，预览里不含表格与公式 -->
    <PublishPreviewDialog
      v-model="previewVisible"
      subset="featured"
      :content="previewContent"
      subtitle="门户访客与移动端学员看到的样子。"
    />
  </div>
</template>

<script setup lang="ts">
import UiIcon from '@/components/ui/UiIcon.vue'
import { ref, reactive, computed, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import type { FormInstance, FormRules } from 'element-plus'
import MarkdownEditor from '@/components/tutor/MarkdownEditor.vue'
import PublishPreviewDialog from '@/components/render/PublishPreviewDialog.vue'
import UiAlert from '@/components/ui/UiAlert.vue'
import { detectOutsideSubset, outsideSubsetNotice } from '@/utils/publishMarkdown'
import { adminFeaturedApi, featuredCategoryOptions } from '@/api/featured'
// #1748：发布时间的三条规则（行 → 状态、表单 → 请求体、提示文案）都是纯函数，单测在 __tests__/FeaturedSchedule.spec.ts
import { buildFeaturedPayload, describePublishTime, editorStatusOf } from './featuredSchedule'
import { resolveFileUrl } from '@/utils/fileUrl'
import { useAsyncPage } from '@/composables/useAsyncPage'
import UiButton from '@/components/ui/UiButton.vue'
import UiUpload from '@/components/ui/UiUpload.vue'
import UiRadioGroup from '@/components/ui/UiRadioGroup.vue'

const route = useRoute()
const router = useRouter()

const formRef = ref<FormInstance | null>(null)
const markdownEditorRef = ref<{ getValue: () => string } | null>(null)
const saving = ref(false)

// 编辑详情加载三态收编 useAsyncPage（#439）：错误仍由拦截器 toast 并回列表，loading 由 composable 驱动
const { loading, run: loadDetail } = useAsyncPage(
  async () => {
    if (!isEdit.value) return null
    try {
      return await adminFeaturedApi.getDetail(editId.value)
    } catch (e) {
      // 错误已由全局拦截器提示；行为冻结：失败回列表（导航是取数轮的后果，留在 loader）
      router.push({ name: 'AdminFeaturedContentList' })
      return null
    }
  },
  {
    // 写回槽（ADR-0069 决策 1）：reactive 表单的八处赋值原先住在 loader 里，同样跑在守卫之前
    apply: (d) => {
      if (!d) return
      form.title = d.title || ''
      form.category = d.category || ''
      form.source = d.source || ''
      form.summary = d.summary || ''
      form.cover_image = d.cover_image || ''
      form.content = d.content || ''
      form.sort_order = d.sort_order || 0
      // 定时项在库里是草稿（靠保持草稿态实现不可见），故必须按 scheduled_at 回填成
      // 「发布 + 该时间」；照抄 status 会让编辑器在保存时把定时取消（#1748 的关键规则）。
      const editorState = editorStatusOf(d)
      form.status = editorState.status
      form.published_at = editorState.publishedAt
    }
  }
)

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
  status: 0 as number,
  // null = 留空 = 保存后立即发布（#1748）；空串是提交面的事，表单里只用 null 表达「没填」
  published_at: null as Date | null
})

/** 提示文案（规则单点在 describePublishTime，与提交面的语义矩阵同源） */
const publishHint = computed(() => describePublishTime(form.published_at))

/** 主按钮文案跟着时间走：定时 / 补录 / 立即 三种说法，避免「点了才发现不是现在发」 */
const saveLabel = computed(() => {
  if (form.status !== 1) return '保存草稿'
  if (!form.published_at) return '保存并发布'
  return form.published_at.getTime() > Date.now() ? '保存并定时' : '保存并发布'
})

function useNow() {
  form.published_at = new Date()
}

// 子集越界提示（#902）：判据与渲染同源（见 publishMarkdown.detectOutsideSubset）
const outsideSubsetLabels = computed(() => detectOutsideSubset(form.content))

/** 编辑器提示与预览里的说明同源（publishMarkdown.outsideSubsetNotice） */
const outsideNotice = computed(() => outsideSubsetNotice(outsideSubsetLabels.value))

// 发布端预览（#903）：同讲师端，取编辑器最新值快照，不写回表单、不发请求
const previewVisible = ref(false)
const previewContent = ref('')

function openPreview() {
  previewContent.value = markdownEditorRef.value?.getValue() || form.content
  previewVisible.value = true
}

const rules: FormRules = {
  title: [
    { required: true, message: '请输入文章标题', trigger: 'blur' },
    { min: 2, max: 200, message: '标题长度为 2-200 个字符', trigger: 'blur' }
  ],
  category: [
    { required: true, message: '请选择分类', trigger: 'change' }
  ]
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
  let latestContent = form.content
  if (markdownEditorRef.value) {
    const latest = markdownEditorRef.value.getValue()
    if (latest) {
      form.content = latest
      latestContent = latest
    }
  }

  if (!latestContent.trim()) {
    ElMessage.warning('正文内容不能为空')
    return
  }

  saving.value = true
  // 提交规则单点在 buildFeaturedPayload：草稿不带时间、发布带时间（空串 = 用现在）
  const payload = buildFeaturedPayload(form, { content: latestContent })
  try {
    if (isEdit.value) {
      await adminFeaturedApi.update(editId.value, payload)
      ElMessage.success(savedMessage('已保存'))
      router.push({ name: 'AdminFeaturedContentList' })
    } else {
      await adminFeaturedApi.create(payload)
      ElMessage.success(savedMessage('已创建'))
      router.push({ name: 'AdminFeaturedContentList' })
    }
  } catch (e: any) {
    // 错误已由全局拦截器提示
  } finally {
    saving.value = false
  }
}

/** 保存成功的说法：定时与补录都不是「已发布」，别让提示替用户判断结果 */
function savedMessage(prefix: string): string {
  if (form.status !== 1) return `${prefix}草稿`
  if (form.published_at && form.published_at.getTime() > Date.now()) return `${prefix}，已排期定时发布`
  return `${prefix}并发布`
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
  /* 卡片底色用「表面」token：--color-text-inverse 在深色下仍是白，等于把整张卡刷成白板 */
  background: var(--color-bg-card);
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
  color: var(--color-text-tertiary);
  font-size: 13px;
  background: var(--color-bg-page);
}

.cover-placeholder .el-icon {
  font-size: 28px;
}

.cover-actions {
  margin-top: 8px;
}

.publish-time-group {
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  gap: 6px;
  width: 100%;
}

.publish-time {
  display: flex;
  align-items: center;
  gap: 8px;
}

.publish-hint {
  font-size: 12px;
  line-height: 1.6;
  color: var(--color-text-tertiary);
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
