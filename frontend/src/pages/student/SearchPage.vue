<template>
  <div class="mx-auto max-w-[960px] p-5">
    <div class="mb-4">
      <h2 class="text-[22px] text-ink">全局搜索</h2>
    </div>

    <div class="mb-5 flex gap-3">
      <el-input
        ref="inputRef"
        v-model="keyword"
        size="large"
        placeholder="搜索课程 / 章节 / 题目 / 内容精选 / 帖子"
        clearable
        class="flex-1"
        @keyup.enter="submitSearch"
        @clear="resetSearch"
      >
        <template #prefix>
          <el-icon><Search /></el-icon>
        </template>
      </el-input>
      <UiButton variant="primary" size="large" :loading="loading" @click="submitSearch">搜索</UiButton>
    </div>

    <!-- 未搜索态（空态两级之一）：本地历史 + 输入提示；历史只在端上（ADR-0049 决策 7） -->
    <div v-if="!searched" class="rounded-card bg-panel p-5 shadow-card">
      <template v-if="history.length > 0">
        <div class="mb-3 flex items-center justify-between">
          <span class="text-[15px] font-semibold text-ink">搜索历史</span>
          <UiButton variant="text" size="small" @click="onClearHistory">清空</UiButton>
        </div>
        <div class="flex flex-wrap gap-2">
          <span
            v-for="kw in history"
            :key="kw"
            class="inline-flex items-center gap-1.5 rounded-full bg-canvas px-3 py-1 text-[13px] text-ink-2"
          >
            <span class="cursor-pointer" @click="searchWith(kw)">{{ kw }}</span>
            <el-icon class="cursor-pointer text-ink-muted hover:text-bad" @click="onRemoveHistory(kw)"><Close /></el-icon>
          </span>
        </div>
      </template>
      <p v-else class="text-[13px] text-ink-3">输入关键词，搜索课程、章节正文、题目、内容精选与帖子</p>
    </div>

    <template v-else>
      <UiSegmentTabs
        :model-value="activeType"
        :options="typeTabOptions"
        class="mb-3"
        @update:model-value="onTypeChange"
      />

      <div class="min-h-[200px] rounded-card bg-panel px-5 pb-4 shadow-card">
        <UiAsyncSection
          :error="loadError"
          :loading="loading"
          :retrying="retrying"
          :skeleton="false"
          error-title="搜索失败"
          error-description="网络或服务端异常，可重试"
          @retry="retry"
        >
          <!-- 首屏才骨架（有结果后翻页/追加不闪——原 loading && !hasAnyResult 口径）；
               空态展示（全部模式/单模式）留在内容区内部，口径与原实现一致 -->
          <UiSkeleton v-if="loading && !hasAnyResult" variant="list" :count="6" />

          <template v-else>
          <!-- 全部模式：分区并列（ADR-0049 决策 5），区内已由后端排好 -->
          <template v-if="activeType === 'all' && allResult">
            <template v-if="!isAllEmpty">
            <div v-for="section in sections" :key="section.key" class="border-b border-line py-3.5 last:border-b-0">
              <div class="mb-2 flex items-baseline gap-2.5">
                <span class="text-[15px] font-semibold text-ink">{{ section.label }}</span>
                <span class="text-xs text-ink-3">{{ section.data.total }} 条</span>
                <span
                  v-if="section.data.total > section.data.items.length"
                  class="ml-auto cursor-pointer text-[13px] text-ui-600"
                  @click="onTypeChange(section.key)"
                >查看全部</span>
              </div>
              <template v-if="section.data.items.length > 0">
                <div
                  v-for="item in section.data.items"
                  :key="item.type + '-' + item.id"
                  class="rounded-[6px] px-2.5 py-2 transition-colors duration-[var(--duration-base)] ease-[var(--ease-default)]"
                  :class="itemTarget(item) ? 'cursor-pointer hover:bg-canvas' : ''"
                  @click="goItem(item)"
                >
                  <SearchResultRow :item="item" :keyword="keyword" />
                </div>
              </template>
              <div v-else class="py-1 text-[13px] text-ink-muted">无匹配结果</div>
            </div>
            </template>
            <UiEmptyState v-else description="没有找到相关内容">
              <div class="mt-2 flex gap-2">
                <UiButton size="small" @click="goPath('/training/question-bank')">去题库练习</UiButton>
                <UiButton size="small" @click="goPath('/training/forum')">去论坛提问</UiButton>
              </div>
            </UiEmptyState>
          </template>

          <!-- 指定类型：分页列表 -->
          <template v-else-if="pageResult">
            <template v-if="pageResult.items.length > 0">
              <div
                v-for="item in pageResult.items"
                :key="item.type + '-' + item.id"
                class="rounded-[6px] px-2.5 py-2 transition-colors duration-[var(--duration-base)] ease-[var(--ease-default)]"
                :class="itemTarget(item) ? 'cursor-pointer hover:bg-canvas' : ''"
                @click="goItem(item)"
              >
                <SearchResultRow :item="item" :keyword="keyword" />
              </div>
            </template>
            <UiEmptyState v-else description="没有找到相关内容">
              <div class="mt-2 flex gap-2">
                <UiButton size="small" @click="goPath('/training/question-bank')">去题库练习</UiButton>
                <UiButton size="small" @click="goPath('/training/forum')">去论坛提问</UiButton>
              </div>
            </UiEmptyState>

            <div v-if="total > pageSize" class="mt-4 flex justify-center">
              <UiPagination
                :current-page="currentPage"
                :page-size="pageSize"
                :total="total"
                @current-change="onPageChange"
              />
            </div>
          </template>
        </template>
        </UiAsyncSection>
      </div>
    </template>
  </div>
</template>

<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { Close, Search } from '@element-plus/icons-vue'
import { searchApi, type SearchAllResult, type SearchItem, type SearchPageResult, type SearchType } from '@/api/search'
import { contentObjectBySearchType, searchableContentObjects, type ContentObjectTarget } from '@/config/contentObjects'
import { useAsyncPage } from '@/composables/useAsyncPage'
import { useCredentialStore } from '@/stores/credential'
import { clearSearchHistory, loadSearchHistory, pushSearchHistory, removeSearchHistory } from '@/utils/searchHistory'
import SearchResultRow from '@/components/student/SearchResultRow.vue'
import UiEmptyState from '@/components/ui/UiEmptyState.vue'
import UiAsyncSection from '@/components/ui/UiAsyncSection.vue'
import UiSkeleton from '@/components/ui/UiSkeleton.vue'
import UiButton from '@/components/ui/UiButton.vue'
import UiSegmentTabs from '@/components/ui/UiSegmentTabs.vue'
import UiPagination from '@/components/ui/UiPagination.vue'

const route = useRoute()
const router = useRouter()
const credentialStore = useCredentialStore()

type TypeKey = 'all' | SearchType
const VALID_TYPES: SearchType[] = ['course', 'chapter', 'question', 'content', 'topic']

const keyword = ref('')
const searched = ref(false)
/** 输入框 ref：供 ⌘/Ctrl+K 跳到本页后聚焦（#984） */
const inputRef = ref<{ focus?: () => void } | null>(null)
const activeType = ref<TypeKey>('all')
const history = ref<string[]>(loadSearchHistory())

const allResult = ref<SearchAllResult | null>(null)
const pageResult = ref<SearchPageResult | null>(null)
// 各分区命中数：聚合搜索时拿到并**留住**——切到指定类型后 tab 计数不该消失
const sectionTotals = ref<Record<string, number>>({})

// 三态 + 分页三件套（#388）：loader 按 activeType 分流（聚合 / 指定类型分页），
// retry 因此天然回到触发失败的那次查询。
const { loading, loadError, retrying, retry, page: currentPage, pageSize, total, run } = useAsyncPage(async () => {
  const kw = keyword.value.trim()
  if (!kw) return
  // 「浏览指定证件」语义：/search 是无 JWT 的公开端点，匿名/非学员角色服务端不兜底（按不分区处理），
  // 要按证件分区只能显式下发；未选证件时本值本就是 undefined（不传 = 不分区，与兜底无关）。
  const browseCredentialId = credentialStore.current?.id ?? undefined
  if (activeType.value === 'all') {
    const res = (await searchApi.search({ keyword: kw, credential_id: browseCredentialId })) as SearchAllResult
    allResult.value = res
    pageResult.value = null
    sectionTotals.value = {
      course: res.courses.total,
      chapter: res.chapters.total,
      question: res.questions.total,
      content: res.contents.total,
      topic: res.topics.total
    }
  } else {
    const res = (await searchApi.search({
      keyword: kw,
      type: activeType.value,
      page: currentPage.value,
      page_size: pageSize.value,
      credential_id: browseCredentialId
    })) as SearchPageResult
    pageResult.value = res
    allResult.value = null
    total.value = res.total || 0
  }
})

// 分区与落点全部派生自内容对象声明表（票 2，#1168）——「种类→称谓/落点」不再在本页手抄。
const sections = computed(() => {
  const r = allResult.value
  if (!r) return []
  return searchableContentObjects().map(o => ({
    key: o.searchType as SearchType,
    label: o.label,
    data: r[o.searchField!]
  }))
})

// 类型 tab 计数：聚合响应本来就带每分区 total（候选 W5），不必额外请求。
const typeTabOptions = computed(() => {
  const hasCounts = searched.value && Object.keys(sectionTotals.value).length > 0
  const count = (key: string) => (hasCounts ? '(' + (sectionTotals.value[key] ?? 0) + ')' : '')
  return [
    { label: '全部', value: 'all' },
    ...searchableContentObjects().map(o => ({ label: o.label + count(o.searchType as string), value: o.searchType as SearchType }))
  ]
})

const hasAnyResult = computed(() => allResult.value !== null || pageResult.value !== null)
const isAllEmpty = computed(() => {
  const r = allResult.value
  if (!r) return false
  return r.courses.items.length === 0 && r.chapters.items.length === 0 && r.questions.items.length === 0 &&
    r.contents.items.length === 0 && r.topics.items.length === 0
})

/** 落点：每条结果都必须能打开（ADR-0049 决策 2 的落点判据）；装配在内容对象表（票 2） */
function itemTarget(item: SearchItem): ContentObjectTarget | null {
  const o = contentObjectBySearchType(item.type as SearchType)
  return o ? o.to({ id: item.id, parentId: item.parent_id }) : null
}

function goItem(item: SearchItem) {
  const target = itemTarget(item)
  if (target) void router.push(target)
}

function goPath(path: string) {
  void router.push(path)
}

/** URL 状态同步（候选 W2）：刷新、后退、分享都能复现同一次搜索 */
function syncUrl() {
  const query: Record<string, string> = {}
  const kw = keyword.value.trim()
  if (kw) query.keyword = kw
  if (activeType.value !== 'all') query.type = activeType.value
  if (currentPage.value > 1) query.page = String(currentPage.value)
  void router.replace({ query })
}

async function submitSearch() {
  const kw = keyword.value.trim()
  if (!kw) {
    resetSearch()
    return
  }
  history.value = pushSearchHistory(kw)
  searched.value = true
  activeType.value = 'all'
  currentPage.value = 1
  syncUrl()
  await run()
}

function searchWith(kw: string) {
  keyword.value = kw
  void submitSearch()
}

function onTypeChange(value: string) {
  const next = value as TypeKey
  if (next === activeType.value) return
  activeType.value = next
  currentPage.value = 1
  syncUrl()
  void run()
}

function onPageChange(page: number) {
  currentPage.value = page
  syncUrl()
  void run()
}

function resetSearch() {
  keyword.value = ''
  searched.value = false
  activeType.value = 'all'
  currentPage.value = 1
  allResult.value = null
  pageResult.value = null
  syncUrl()
}

function onRemoveHistory(kw: string) {
  history.value = removeSearchHistory(kw)
}

function onClearHistory() {
  clearSearchHistory()
  history.value = []
}

function normalizeType(raw: unknown): TypeKey {
  return typeof raw === 'string' && (VALID_TYPES as string[]).includes(raw) ? (raw as SearchType) : 'all'
}

/** 首屏从 URL 复现搜索态；带 query 打开即直接出结果；⌘/Ctrl+K 带 focus=1 时聚焦输入框 */
function focusInput() {
  void nextTick(() => inputRef.value?.focus?.())
}

onMounted(async () => {
  window.addEventListener('focus-global-search', focusInput)
  if (route.query.focus === '1') {
    focusInput()
  }
  const kw = typeof route.query.keyword === 'string' ? route.query.keyword : ''
  if (!kw) return
  keyword.value = kw
  activeType.value = normalizeType(route.query.type)
  currentPage.value = Number(route.query.page ?? 1) || 1
  searched.value = true
  await run()
})

onBeforeUnmount(() => {
  window.removeEventListener('focus-global-search', focusInput)
})

// 浏览器前进/后退：只在与当前输入不一致时重装，避免与 replace 形成回环
watch(
  () => route.query.keyword,
  (raw) => {
    const kw = typeof raw === 'string' ? raw : ''
    if (kw === keyword.value) return
    keyword.value = kw
    if (!kw) {
      resetSearch()
      return
    }
    activeType.value = normalizeType(route.query.type)
    currentPage.value = Number(route.query.page ?? 1) || 1
    searched.value = true
    void run()
  }
)
</script>
