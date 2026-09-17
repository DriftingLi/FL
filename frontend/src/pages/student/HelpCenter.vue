<template>
  <div class="mx-auto max-w-[960px]">
    <h2 class="mb-1 mt-0 text-2xl font-semibold text-ink">帮助中心</h2>
    <p class="mb-4 mt-0 text-[13px] text-ink-3">先搜关键词，或从左侧分类里找。</p>

    <el-input v-model="keyword" class="mb-4" placeholder="搜索问题关键词" clearable />

    <UiAsyncSection
      :error="loadError"
      :loading="loading"
      :empty="isEmpty"
      :retrying="retrying"
      error-title="帮助内容加载失败"
      error-description="网络或服务端异常，可重试"
      @retry="retryLoad"
    >
      <template #skeleton>
        <UiSkeleton variant="card" :count="3" />
      </template>

      <div class="flex items-start gap-5">
        <!-- 左侧分类导航：搜索时导航不参与筛选（命中项可能跨分类），但仍显示各处命中数 -->
        <nav class="w-[168px] shrink-0 rounded-card border border-line bg-panel p-1.5">
          <button
            v-for="c in categoryNav"
            :key="c.code"
            type="button"
            class="flex w-full items-center justify-between rounded-ctl border-0 bg-transparent px-2.5 py-2 text-left text-[13px] transition-colors duration-150"
            :class="c.active ? 'bg-ui-50 font-semibold text-ui-600' : 'text-ink-2 hover:bg-canvas'"
            @click="selectCategory(c.code)"
          >
            <span class="truncate">{{ c.title }}</span>
            <span class="ml-2 shrink-0 text-ink-3">{{ c.count }}</span>
          </button>
        </nav>

        <!-- 右侧问答 -->
        <div class="min-w-0 flex-1">
          <UiEmptyState v-if="visibleGroups.length === 0" description="没有匹配的问题" />
          <section
            v-for="g in visibleGroups"
            :key="g.code"
            class="mb-3 overflow-hidden rounded-card border border-line bg-panel"
          >
            <div v-if="g.title" class="border-b border-line px-4 py-2.5 text-[13px] font-semibold text-ink-3">
              {{ g.title }}
            </div>
            <el-collapse class="help-collapse">
              <el-collapse-item v-for="e in g.entries" :key="e.id" :title="e.question" :name="String(e.id)">
                <p class="m-0 whitespace-pre-wrap text-[14px] leading-[1.7] text-ink-2">{{ e.answer }}</p>
              </el-collapse-item>
            </el-collapse>
          </section>
        </div>
      </div>

      <template #empty>
        <UiEmptyState description="帮助内容正在准备中" />
      </template>
    </UiAsyncSection>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { faqApi, type HelpCenter } from '@/api/faq'
import { useAsyncPage } from '@/composables/useAsyncPage'
import UiAsyncSection from '@/components/ui/UiAsyncSection.vue'
import UiEmptyState from '@/components/ui/UiEmptyState.vue'
import UiSkeleton from '@/components/ui/UiSkeleton.vue'

const center = ref<HelpCenter>({ categories: [] })
const keyword = ref('')
// 空串 = 全部分类；搜索时忽略该选择（命中项可能跨分类）
const activeCode = ref('')

const {
  loading,
  loadError,
  retrying,
  retry: retryLoad,
  isEmpty,
  run: loadData
} = useAsyncPage(async () => {
  const res = await faqApi.getHelpCenter()
  center.value = res || { categories: [] }
}, { itemsRef: center })

const categories = computed(() => center.value.categories || [])

/** 端上搜索：问题或答案命中即算（不区分大小写，去首尾空白） */
function matches(question: string, answer: string): boolean {
  const k = keyword.value.trim().toLowerCase()
  if (!k) return true
  return question.toLowerCase().includes(k) || answer.toLowerCase().includes(k)
}

/** 左侧分类导航：条目数按**当前搜索命中数**显示 */
const categoryNav = computed(() => {
  const searching = keyword.value.trim().length > 0
  const countOf = (entries: { question: string; answer: string }[]) =>
    (entries || []).filter((e) => matches(e.question, e.answer)).length
  const total = categories.value.reduce((sum, c) => sum + countOf(c.entries), 0)
  const nav = [{ code: '', title: '全部', count: total, active: activeCode.value === '' && !searching }]
  for (const c of categories.value) {
    nav.push({ code: c.code, title: c.title, count: countOf(c.entries), active: activeCode.value === c.code && !searching })
  }
  return nav
})

/** 右侧展示：搜索中跨分类展示全部命中；否则只展示选中分类 */
const visibleGroups = computed(() => {
  const searching = keyword.value.trim().length > 0
  const groups = categories.value
    .filter((c) => searching || activeCode.value === '' || c.code === activeCode.value)
    .map((c) => ({
      code: c.code,
      title: c.title,
      entries: (c.entries || []).filter((e) => matches(e.question, e.answer))
    }))
    .filter((g) => g.entries.length > 0)
  // 只有一个分类有内容时不重复显示分类标题（搜索命中窄时尤其啰嗦）
  return groups.length === 1 ? [{ ...groups[0], title: '' }] : groups
})

function selectCategory(code: string) {
  activeCode.value = code
  keyword.value = ''
}

onMounted(() => {
  void loadData()
})
</script>

<style scoped>
/* Element Plus 手风琴默认是「上下通栏分隔线 + 无圆角」，与卡片容器重复。
   这里用 :deep() 收掉它自己的边框，让卡片负责外框（AGENTS.md R3：覆盖 EP 外观二选一，此处走 :deep()）。 */
.help-collapse :deep(.el-collapse),
.help-collapse :deep(.el-collapse-item__wrap) {
  border-bottom: none;
}

.help-collapse :deep(.el-collapse-item__header) {
  height: auto;
  min-height: 46px;
  padding: 0 16px;
  border-bottom-color: var(--color-line);
  font-size: 14px;
  font-weight: var(--font-medium);
  line-height: 1.5;
  color: var(--color-text-primary);
}

.help-collapse :deep(.el-collapse-item:last-child .el-collapse-item__header) {
  border-bottom-color: transparent;
}

.help-collapse :deep(.el-collapse-item__content) {
  padding: 0 16px 14px;
}
</style>
