<template>
  <div class="mx-auto max-w-[960px] p-5">
    <div class="mb-4">
      <h2 class="text-[22px] text-ink">全局搜索</h2>
    </div>

    <div class="mb-5 flex gap-3">
      <el-input
        v-model="keyword"
        size="large"
        placeholder="搜索课程 / 题目 / 资讯 / 帖子"
        clearable
        class="flex-1"
        @keyup.enter="doSearch"
        @clear="resetSearch"
      >
        <template #prefix>
          <el-icon><Search /></el-icon>
        </template>
      </el-input>
      <UiButton variant="primary" size="large" :loading="loading" @click="doSearch">搜索</UiButton>
    </div>

    <template v-if="searched">
      <!-- #511：分类 tab 统一分段控件 -->
      <UiSegmentTabs
        :model-value="activeType"
        :options="typeTabOptions"
        @update:model-value="(v: string) => { activeType = v as 'all' | SearchType; handleTabChange() }"
        class="mb-3"
      />

      <div class="min-h-[200px] rounded-card bg-panel px-5 pb-4 shadow-card">
        <UiErrorState
          v-if="loadError"
          title="搜索失败"
          description="网络或服务端异常，可重试"
          :retrying="retrying"
          @retry="retryLoad"
        />

        <UiSkeleton v-else-if="loading" variant="list" :count="6" />

        <template v-else>
          <!-- 全部模式：四分区 -->
          <template v-if="activeType === 'all' && allResult">
            <div
              v-for="section in sections"
              :key="section.key"
              class="border-b border-line py-3.5 last:border-b-0"
            >
              <div class="mb-2 flex items-baseline gap-2.5">
                <span class="text-[15px] font-semibold text-ink">{{ section.label }}</span>
                <span class="text-xs text-ink-3">{{ section.data.total }} 条</span>
              </div>
              <template v-if="section.data.items.length > 0">
                <div
                  v-for="item in section.data.items"
                  :key="`${item.type}-${item.id}`"
                  class="flex items-baseline gap-2.5 rounded-[6px] px-2.5 py-2"
                  :class="
                    itemPath(item)
                      ? 'cursor-pointer transition-colors duration-[var(--duration-base)] ease-[var(--ease-default)] hover:bg-canvas'
                      : ''
                  "
                  @click="goItem(item)"
                >
                  <span class="max-w-[50%] shrink-0 truncate text-sm text-ink">{{ item.title }}</span>
                  <span v-if="item.summary" class="truncate text-[13px] text-ink-3">{{ item.summary }}</span>
                </div>
              </template>
              <div v-else class="py-1 text-[13px] text-ink-muted">无匹配结果</div>
            </div>
          </template>

          <!-- 指定类型模式：分页列表 -->
          <template v-else-if="pageResult">
            <template v-if="pageResult.items.length > 0">
              <div
                v-for="item in pageResult.items"
                :key="`${item.type}-${item.id}`"
                class="flex items-baseline gap-2.5 rounded-[6px] px-2.5 py-2"
                :class="
                  itemPath(item)
                    ? 'cursor-pointer transition-colors duration-[var(--duration-base)] ease-[var(--ease-default)] hover:bg-canvas'
                    : ''
                "
                @click="goItem(item)"
              >
                <span class="max-w-[50%] shrink-0 truncate text-sm text-ink">{{ item.title }}</span>
                <span v-if="item.summary" class="truncate text-[13px] text-ink-3">{{ item.summary }}</span>
              </div>
            </template>
            <UiEmptyState v-else description="无匹配结果" />

            <div class="mt-4 flex justify-center" v-if="total > pageSize">
              <el-pagination
                v-model:current-page="currentPage"
                :page-size="pageSize"
                :total="total"
                layout="total, prev, pager, next"
                @current-change="handlePageChange"
              />
            </div>
          </template>
        </template>
      </div>
    </template>
  </div>
</template>

<script setup lang="ts">
import { ref, computed } from 'vue'
import { useRouter } from 'vue-router'
import { Search } from '@element-plus/icons-vue'
import { searchApi, type SearchAllResult, type SearchPageResult, type SearchItem, type SearchType } from '@/api/search'
import { useAsyncPage } from '@/composables/useAsyncPage'
import UiEmptyState from '@/components/ui/UiEmptyState.vue'
import UiErrorState from '@/components/ui/UiErrorState.vue'
import UiSkeleton from '@/components/ui/UiSkeleton.vue'
import UiButton from '@/components/ui/UiButton.vue'
import UiSegmentTabs from '@/components/ui/UiSegmentTabs.vue'

const router = useRouter()

const keyword = ref('')
const searched = ref(false)
const activeType = ref<'all' | SearchType>('all')

// #511：UiSegmentTabs 选项（全部分类 tab）
const typeTabOptions = [
  { label: '全部', value: 'all' },
  { label: '课程', value: 'course' },
  { label: '题目', value: 'question' },
  { label: '资讯', value: 'content' },
  { label: '帖子', value: 'topic' }
]
const allResult = ref<SearchAllResult | null>(null)
const pageResult = ref<SearchPageResult | null>(null)

// 三态 + 分页三件套收编（#388）：loader 按 activeType 分流（聚合 / 指定类型分页），
// retry 因此天然回到触发失败的那次查询
const {
  loading,
  loadError,
  retrying,
  retry: retryLoad,
  page: currentPage,
  pageSize,
  total,
  run: runSearch,
  handlePageChange
} = useAsyncPage(async () => {
  const kw = keyword.value.trim()
  if (!kw) return
  if (activeType.value === 'all') {
    // type 缺省时后端返回各分区聚合（SearchAllResult）
    allResult.value = (await searchApi.search({ keyword: kw })) as SearchAllResult
    pageResult.value = null
  } else {
    const res = (await searchApi.search({
      keyword: kw,
      type: activeType.value,
      page: currentPage.value,
      page_size: pageSize.value
    })) as SearchPageResult
    pageResult.value = res
    allResult.value = null
    total.value = res.total || 0
  }
})

const sections = computed(() => {
  const r = allResult.value
  if (!r) return []
  return [
    { key: 'course', label: '课程', data: r.courses },
    { key: 'question', label: '题目', data: r.questions },
    { key: 'content', label: '资讯', data: r.contents },
    { key: 'topic', label: '帖子', data: r.topics }
  ]
})

// 可跳转类型：课程 → 课程中心详情（query 打开），帖子 → 论坛详情；题目/资讯仅展示
function itemPath(item: SearchItem): string {
  if (item.type === 'course') {
    return `/training/courses?course_id=${item.id}`
  }
  if (item.type === 'topic') {
    return `/training/forum/${item.id}`
  }
  return ''
}

function goItem(item: SearchItem) {
  const path = itemPath(item)
  if (path) {
    router.push(path)
  }
}

/** 新搜索入口：回到聚合视图并重置页码，装载交给 runSearch（按 activeType 分流） */
async function doSearch() {
  if (!keyword.value.trim()) return
  searched.value = true
  activeType.value = 'all'
  currentPage.value = 1
  await runSearch()
}

function handleTabChange() {
  currentPage.value = 1
  void runSearch()
}

function resetSearch() {
  searched.value = false
  allResult.value = null
  pageResult.value = null
}
</script>
