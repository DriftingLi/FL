<template>
  <div class="mx-auto max-w-[960px]">
    <h2>帮助中心</h2>

    <el-input
      v-model="keyword"
      class="mb-4"
      placeholder="搜索问题关键词"
      clearable
    />

    <UiAsyncSection
      :error="loadError"
      :loading="loading"
      :empty="categories.length === 0"
      :retrying="retrying"
      error-title="帮助内容加载失败"
      error-description="网络或服务端异常，可重试"
      @retry="retryLoad"
    >
      <template #skeleton>
        <UiSkeleton variant="card" :count="3" />
      </template>

      <div class="flex gap-5">
        <!-- 左侧分类：搜索时禁用（命中项可能跨分类，此时右侧按分类分组展示） -->
        <div class="w-[170px] shrink-0">
          <div
            v-for="c in categoryNav"
            :key="c.code"
            class="mb-1 cursor-pointer rounded-card px-3 py-2 text-[13px]"
            :class="c.active ? 'bg-brand-soft font-semibold text-ink' : 'text-ink-2 hover:bg-canvas'"
            @click="selectCategory(c.code)"
          >
            {{ c.title }}
            <span class="ml-1 text-ink-3">{{ c.count }}</span>
          </div>
        </div>

        <!-- 右侧手风琴 -->
        <div class="min-w-0 flex-1">
          <UiEmptyState v-if="visibleGroups.length === 0" description="没有匹配的问题" />
          <div v-for="g in visibleGroups" :key="g.code" class="mb-4">
            <div class="mb-2 text-[13px] font-semibold text-ink-3">{{ g.title }}</div>
            <el-collapse>
              <el-collapse-item v-for="e in g.entries" :key="e.id" :title="e.question" :name="String(e.id)">
                <p class="m-0 whitespace-pre-wrap text-[14px] leading-[1.7] text-ink-2">{{ e.answer }}</p>
              </el-collapse-item>
            </el-collapse>
          </div>
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

const { loading, loadError, retrying, retry: retryLoad, run: loadData } = useAsyncPage(async () => {
  const res = await faqApi.getHelpCenter()
  center.value = res || { categories: [] }
})

const categories = computed(() => center.value.categories || [])

/** 端上搜索：问题或答案命中即算（不区分大小写，去首尾空白） */
function matches(question: string, answer: string): boolean {
  const k = keyword.value.trim().toLowerCase()
  if (!k) return true
  return question.toLowerCase().includes(k) || answer.toLowerCase().includes(k)
}

/** 左侧分类导航：条目数按**当前搜索命中数**显示，搜索时导航不参与筛选 */
const categoryNav = computed(() => {
  const total = categories.value.reduce(
    (sum, c) => sum + (c.entries || []).filter((e) => matches(e.question, e.answer)).length,
    0
  )
  const nav = [{ code: '', title: '全部', count: total, active: activeCode.value === '' && !keyword.value.trim() }]
  for (const c of categories.value) {
    const count = (c.entries || []).filter((e) => matches(e.question, e.answer)).length
    nav.push({ code: c.code, title: c.title, count, active: activeCode.value === c.code && !keyword.value.trim() })
  }
  return nav
})

/** 右侧展示：搜索中跨分类展示全部命中；否则只展示选中分类 */
const visibleGroups = computed(() => {
  const searching = keyword.value.trim().length > 0
  const groups = categories.value
    .filter((c) => searching || activeCode.value === '' || c.code === activeCode.value)
    .map((c) => ({ code: c.code, title: c.title, entries: (c.entries || []).filter((e) => matches(e.question, e.answer)) }))
  const nonEmpty = groups.filter((g) => g.entries.length > 0)
  // 只有一个分类有内容时不重复显示分类标题（搜索命中窄时尤其啰嗦）
  return nonEmpty.length === 1 ? [{ ...nonEmpty[0], title: '' }] : nonEmpty
})

function selectCategory(code: string) {
  activeCode.value = code
  keyword.value = ''
}

onMounted(() => {
  void loadData()
})
</script>
