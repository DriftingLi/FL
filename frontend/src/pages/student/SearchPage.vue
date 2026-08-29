<template>
  <div class="search-page">
    <div class="page-header">
      <h2>全局搜索</h2>
    </div>

    <div class="search-bar">
      <el-input
        v-model="keyword"
        size="large"
        placeholder="搜索课程 / 题目 / 资讯 / 帖子"
        clearable
        @keyup.enter="doSearch"
        @clear="resetSearch"
      >
        <template #prefix>
          <el-icon><Search /></el-icon>
        </template>
      </el-input>
      <el-button type="primary" size="large" :loading="loading" @click="doSearch">搜索</el-button>
    </div>

    <template v-if="searched">
      <el-tabs v-model="activeType" @tab-change="handleTabChange">
        <el-tab-pane label="全部" name="all" />
        <el-tab-pane label="课程" name="course" />
        <el-tab-pane label="题目" name="question" />
        <el-tab-pane label="资讯" name="content" />
        <el-tab-pane label="帖子" name="topic" />
      </el-tabs>

      <div class="search-results">
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
            <div v-for="section in sections" :key="section.key" class="result-section">
              <div class="section-header">
                <span class="section-title">{{ section.label }}</span>
                <span class="section-count">{{ section.data.total }} 条</span>
              </div>
              <template v-if="section.data.items.length > 0">
                <div
                  v-for="item in section.data.items"
                  :key="`${item.type}-${item.id}`"
                  class="result-item"
                  :class="{ clickable: !!itemPath(item) }"
                  @click="goItem(item)"
                >
                  <span class="result-title">{{ item.title }}</span>
                  <span v-if="item.summary" class="result-summary">{{ item.summary }}</span>
                </div>
              </template>
              <div v-else class="section-empty">无匹配结果</div>
            </div>
          </template>

          <!-- 指定类型模式：分页列表 -->
          <template v-else-if="pageResult">
            <template v-if="pageResult.items.length > 0">
              <div
                v-for="item in pageResult.items"
                :key="`${item.type}-${item.id}`"
                class="result-item"
                :class="{ clickable: !!itemPath(item) }"
                @click="goItem(item)"
              >
                <span class="result-title">{{ item.title }}</span>
                <span v-if="item.summary" class="result-summary">{{ item.summary }}</span>
              </div>
            </template>
            <UiEmptyState v-else description="无匹配结果" />

            <div class="pagination-wrapper" v-if="pageResult.total > pageSize">
              <el-pagination
                v-model:current-page="currentPage"
                :page-size="pageSize"
                :total="pageResult.total"
                layout="total, prev, pager, next"
                @current-change="loadPageResult"
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
import UiEmptyState from '@/components/ui/UiEmptyState.vue'
import UiErrorState from '@/components/ui/UiErrorState.vue'
import UiSkeleton from '@/components/ui/UiSkeleton.vue'

const router = useRouter()

const keyword = ref('')
const searched = ref(false)
const loading = ref(false)
const loadError = ref(false)
const retrying = ref(false)
const activeType = ref<'all' | SearchType>('all')
const currentPage = ref(1)
const pageSize = ref(20)

const allResult = ref<SearchAllResult | null>(null)
const pageResult = ref<SearchPageResult | null>(null)

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

async function doSearch() {
  const kw = keyword.value.trim()
  if (!kw) return
  searched.value = true
  activeType.value = 'all'
  currentPage.value = 1
  loading.value = true
  loadError.value = false
  try {
    // type 缺省时后端返回各分区聚合（SearchAllResult）
    allResult.value = (await searchApi.search({ keyword: kw })) as SearchAllResult
    pageResult.value = null
  } catch (e) {
    console.error('搜索失败:', e)
    loadError.value = true
    allResult.value = null
    pageResult.value = null
  } finally {
    loading.value = false
  }
}

function handleTabChange() {
  currentPage.value = 1
  if (activeType.value === 'all') {
    doSearch()
  } else {
    loadPageResult()
  }
}

async function loadPageResult() {
  const kw = keyword.value.trim()
  if (!kw || activeType.value === 'all') return
  loading.value = true
  loadError.value = false
  try {
    const res = await searchApi.search({
      keyword: kw,
      type: activeType.value,
      page: currentPage.value,
      page_size: pageSize.value
    })
    pageResult.value = res as SearchPageResult
    allResult.value = null
  } catch (e) {
    console.error('搜索失败:', e)
    loadError.value = true
    pageResult.value = null
  } finally {
    loading.value = false
  }
}

/** 重试：回到触发失败的那次查询（聚合搜索 / 指定类型分页） */
async function retryLoad() {
  if (retrying.value) return
  retrying.value = true
  try {
    if (activeType.value === 'all') {
      await doSearch()
    } else {
      await loadPageResult()
    }
  } finally {
    retrying.value = false
  }
}

function resetSearch() {
  searched.value = false
  allResult.value = null
  pageResult.value = null
}
</script>

<style scoped>
.search-page {
  padding: 20px;
  max-width: 960px;
  margin: 0 auto;
}

.page-header {
  margin-bottom: 16px;
}

.page-header h2 {
  font-size: 22px;
  color: var(--color-text-primary);
}

.search-bar {
  display: flex;
  gap: 12px;
  margin-bottom: 20px;
}

.search-bar .el-input {
  flex: 1;
}

.search-results {
  background: var(--color-bg-card);
  border-radius: 12px;
  box-shadow: 0 2px 12px rgba(0, 0, 0, 0.06);
  min-height: 200px;
  padding: 0 20px 16px;
}

.result-section {
  padding: 14px 0;
  border-bottom: 1px solid var(--color-border-light);
}

.result-section:last-child {
  border-bottom: none;
}

.section-header {
  display: flex;
  align-items: baseline;
  gap: 10px;
  margin-bottom: 8px;
}

.section-title {
  font-size: 15px;
  font-weight: 600;
  color: var(--color-text-primary);
}

.section-count {
  font-size: 12px;
  color: var(--color-text-tertiary);
}

.section-empty {
  font-size: 13px;
  color: var(--color-text-disabled);
  padding: 4px 0;
}

.result-item {
  display: flex;
  align-items: baseline;
  gap: 10px;
  padding: 8px 10px;
  border-radius: 6px;
}

.result-item.clickable {
  cursor: pointer;
  transition: background 0.2s;
}

.result-item.clickable:hover {
  background: var(--color-bg-page);
}

.result-title {
  font-size: 14px;
  color: var(--color-text-primary);
  flex-shrink: 0;
  max-width: 50%;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.result-summary {
  font-size: 13px;
  color: var(--color-text-tertiary);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.pagination-wrapper {
  display: flex;
  justify-content: center;
  margin-top: 16px;
}
</style>
