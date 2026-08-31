<template>
  <div class="mx-auto max-w-[760px] px-4 pb-10">
    <div class="mb-3">
      <UiButton variant="text" :icon="ArrowLeft" @click="goBack">返回问答</UiButton>
    </div>

    <UiCard padding="lg">
      <h1 class="m-0 mb-5 text-xl font-semibold text-ink">我要提问</h1>

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

      <div class="flex">
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

