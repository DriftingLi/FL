<template>
  <div class="mx-auto max-w-[960px] p-5">
    <div class="mb-3">
      <UiButton variant="text" size="small" @click="goBack">
        <el-icon><ArrowLeft /></el-icon>
        <span class="ml-1">返回</span>
      </UiButton>
    </div>

    <UiAsyncSection
      :error="loadError"
      :loading="loading"
      :empty="false"
      :retrying="retrying"
      error-title="内容加载失败"
      error-description="内容可能已下架，或网络异常"
      @retry="retry"
    >
      <template #skeleton>
        <UiSkeleton variant="card" :count="1"  />
      </template>
      <template v-if="detail">
      <h1 class="text-[22px] leading-[1.4] text-ink">{{ detail.title }}</h1>
      <div class="mt-2 flex items-center gap-2.5 text-[13px] text-ink-3">
        <UiTag tone="primary" size="small">{{ detail.category_label }}</UiTag>
        <span v-if="detail.source">{{ detail.source }}</span>
        <span>{{ publishedText }}</span>
      </div>
      <img v-if="detail.cover_image" :src="detail.cover_image" alt="" class="mt-4 w-full rounded-card" />

      <!-- 内容精选属可信面，正文子集取三端交集（ADR-0046）：subset=featured -->
      <PublishMarkdown :content="detail.content" subset="featured" class="mt-4 rounded-card bg-panel p-5 shadow-card" />

      <section v-if="detail.related.length > 0" class="mt-4 rounded-card bg-panel p-5 shadow-card">
        <h2 class="mb-2 text-[15px] font-semibold text-ink">相关阅读</h2>
        <div
          v-for="rel in detail.related"
          :key="rel.content_id"
          class="cursor-pointer rounded-[6px] px-2 py-2 text-sm text-ink hover:bg-canvas"
          @click="open(rel.content_id)"
        >{{ rel.title }}</div>
      </section>

      <div class="mt-4 flex justify-between gap-3">
        <UiButton v-if="detail.prev.content_id > 0" variant="ghost" @click="open(detail.prev.content_id)">
          上一篇：{{ detail.prev.title }}
        </UiButton>
        <UiButton v-if="detail.next.content_id > 0" variant="ghost" @click="open(detail.next.content_id)">
          下一篇：{{ detail.next.title }}
        </UiButton>
      </div>
      </template>
    </UiAsyncSection>
  </div>
</template>

<script setup lang="ts">
// 内容精选详情（ADR-0049 决策 4）：搜索结果与收藏页的「内容精选」落点，
// 自包含在训练域内 —— 不把学员带去官网门户（独立仓，且会离开工作区）。
import { computed, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ArrowLeft } from '@element-plus/icons-vue'
import { featuredApi } from '@/api/featured'
import type { FeaturedContentDetailDTO } from '@/api/generated/featured'
import { useAsyncPage } from '@/composables/useAsyncPage'
import PublishMarkdown from '@/components/render/PublishMarkdown.vue'
import UiButton from '@/components/ui/UiButton.vue'
import UiTag from '@/components/ui/UiTag.vue'
import UiAsyncSection from '@/components/ui/UiAsyncSection.vue'
import UiSkeleton from '@/components/ui/UiSkeleton.vue'

const route = useRoute()
const router = useRouter()

const detail = ref<FeaturedContentDetailDTO | null>(null)

const { loading, loadError, retrying, retry, run } = useAsyncPage(async () => {
  const id = Number(route.params.id)
  if (!id) return
  detail.value = await featuredApi.getDetail(id)
})

const publishedText = computed(() => ((detail.value?.published_at ?? detail.value?.created_at) ?? '').slice(0, 10))

function goBack() {
  router.back()
}

function open(id: number) {
  if (id > 0) void router.push('/training/featured/' + id)
}

onMounted(() => {
  void run()
})

// 同页换 id（相关阅读 / 上下篇点击）时重装
watch(
  () => route.params.id,
  () => {
    void run()
  }
)
</script>
