<template>
  <div class="ask-page">
    <div class="back-bar">
      <el-button text :icon="ArrowLeft" @click="goBack">返回问答</el-button>
    </div>

    <div class="ask-card">
      <h1 class="ask-title">我要提问</h1>
      <p class="ask-hint">清晰的问题更容易获得解答</p>

      <el-form label-width="56px" @submit.prevent>
        <el-form-item label="标题" required>
          <el-input
            v-model="form.title"
            maxlength="100"
            show-word-limit
            placeholder="一句话概括你的问题（1-100 字）"
          />
        </el-form-item>

        <el-form-item label="正文" required>
          <el-input
            v-model="form.content"
            type="textarea"
            :rows="10"
            maxlength="10000"
            show-word-limit
            placeholder="详细描述你的问题、已尝试的方法、出错信息等（1-10000 字，纯文本）"
          />
        </el-form-item>

        <el-form-item label="图片">
          <ForumImageUploader v-model="form.images" :max="9" />
        </el-form-item>

        <el-form-item>
          <el-button type="primary" :loading="submitting" :disabled="!canSubmit" @click="submit">发布提问</el-button>
          <el-button @click="goBack">取消</el-button>
        </el-form-item>
      </el-form>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { ArrowLeft } from '@element-plus/icons-vue'
import { forumApi } from '@/api/forum'
import ForumImageUploader from '@/components/student/ForumImageUploader.vue'

const router = useRouter()

const form = ref<{ title: string; content: string; images: string[] }>({ title: '', content: '', images: [] })
const submitting = ref(false)

const canSubmit = computed(() => form.value.title.trim().length > 0 && form.value.content.trim().length > 0)

function goBack() {
  // 返回问答 Tab
  router.push({ name: 'ForumPage', query: { tab: 'question' } })
}

async function submit() {
  const title = form.value.title.trim()
  const content = form.value.content.trim()
  if (!title || !content) {
    ElMessage.warning('请填写标题和正文')
    return
  }
  if (title.length > 100) {
    ElMessage.warning('标题不能超过 100 字')
    return
  }
  if (content.length > 10000) {
    ElMessage.warning('正文不能超过 10000 字')
    return
  }
  submitting.value = true
  try {
    // 问答帖不带 chapter_id，显式指定 category
    await forumApi.createTopic({ category: 'question', title, content, images: form.value.images })
    ElMessage.success('提问发布成功')
    router.push({ name: 'ForumPage', query: { tab: 'question' } })
  } catch (e) {
    console.error('发布提问失败:', e)
  } finally {
    submitting.value = false
  }
}
</script>

<style scoped>
.ask-page {
  max-width: 760px;
  margin: 0 auto;
  padding: 0 16px 40px;
}

.back-bar {
  margin-bottom: 12px;
}

.ask-card {
  background: var(--color-bg-card);
  border-radius: 12px;
  box-shadow: 0 2px 12px rgba(0, 0, 0, 0.06);
  padding: 24px;
}

.ask-title {
  font-size: 20px;
  font-weight: 600;
  color: var(--color-text-primary);
  margin: 0 0 4px;
}

.ask-hint {
  font-size: 13px;
  color: var(--color-text-tertiary);
  margin: 0 0 20px;
}
</style>
