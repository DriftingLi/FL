<template>
  <div class="ask-page">
    <div class="back-bar">
      <UiButton variant="text" :icon="ArrowLeft" @click="goBack">返回问答</UiButton>
    </div>

    <UiCard padding="lg">
      <h1 class="ask-title">我要提问</h1>

      <ForumPostForm
        ref="postForm"
        category="question"
        label-width="56px"
        content-label="正文"
        :content-rows="10"
        title-placeholder="一句话概括你的问题（1-100 字）"
        content-placeholder="详细描述你的问题、已尝试的方法、出错信息等（1-10000 字，纯文本）"
        success-message="提问发布成功"
        @success="goBack"
      />

      <div class="ask-actions">
        <UiButton variant="primary" :loading="postForm?.submitting" :disabled="!postForm?.canSubmit" @click="postForm?.submit()">发布提问</UiButton>
        <UiButton @click="goBack">取消</UiButton>
      </div>
    </UiCard>
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { ArrowLeft } from '@element-plus/icons-vue'
import ForumPostForm from '@/components/student/ForumPostForm.vue'
import UiButton from '@/components/ui/UiButton.vue'
import UiCard from '@/components/ui/UiCard.vue'

// 表单体（#389）：字段/校验/提交在 ForumPostForm（category=question），本页只留整页壳与跳转
const router = useRouter()
const postForm = ref<InstanceType<typeof ForumPostForm> | null>(null)

function goBack() {
  // 返回问答 Tab
  router.push({ name: 'ForumPage', query: { tab: 'question' } })
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

.ask-title {
  font-size: 20px;
  font-weight: 600;
  color: var(--color-text-primary);
  margin: 0 0 20px;
}

.ask-actions {
  display: flex;
}
</style>
