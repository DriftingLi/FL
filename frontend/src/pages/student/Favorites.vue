<template>
  <div class="favorites-page">
    <div class="page-header">
      <h2>我的收藏</h2>
    </div>

    <el-tabs v-model="activeType" @tab-change="handleTabChange">
      <el-tab-pane label="全部" name="all" />
      <el-tab-pane label="课程" name="course" />
      <el-tab-pane label="题目" name="question" />
      <el-tab-pane label="帖子" name="topic" />
    </el-tabs>

    <div class="favorite-list">
      <UiErrorState
        v-if="loadError"
        title="收藏加载失败"
        description="网络或服务端异常，可重试"
        :retrying="retrying"
        @retry="retryLoad"
      />

      <UiSkeleton v-else-if="loading" variant="list" :count="5" />

      <template v-else-if="favorites.length > 0">
        <div
          v-for="(item, i) in favorites"
          :key="item.favorite_id"
          class="favorite-item stagger-in"
          :class="{ clickable: !!itemPath(item) }"
          :style="staggerStyle(i)"
          @click="goItem(item)"
        >
          <el-image
            v-if="item.cover"
            :src="resolveFileUrl(item.cover)"
            fit="cover"
            class="item-cover"
          >
            <template #error>
              <div class="item-cover item-cover-fallback">{{ typeLabel(item.target_type).charAt(0) }}</div>
            </template>
          </el-image>
          <div v-else class="item-cover item-cover-fallback">{{ typeLabel(item.target_type).charAt(0) }}</div>

          <div class="item-main">
            <div class="item-title-row">
              <el-tag size="small" :type="typeTagColor(item.target_type)" effect="plain">
                {{ typeLabel(item.target_type) }}
              </el-tag>
              <span class="item-title">{{ item.title || `${typeLabel(item.target_type)} #${item.target_id}` }}</span>
            </div>
            <span v-if="item.created_at" class="item-time">{{ formatLocaleDateTime(item.created_at) }}</span>
          </div>

          <div class="item-actions" @click.stop>
            <el-button type="danger" text size="small" @click="removeFavorite(item)">移除</el-button>
          </div>
        </div>
      </template>
      <UiEmptyState v-else description="暂无收藏" />
    </div>

    <div class="pagination-wrapper" v-if="total > pageSize">
      <el-pagination
        v-model:current-page="currentPage"
        :page-size="pageSize"
        :total="total"
        layout="total, prev, pager, next"
        @current-change="loadFavorites"
      />
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import { favoriteApi, type FavoriteItem, type FavoriteTargetType } from '@/api/favorite'
import { resolveFileUrl } from '@/utils/fileUrl'
import { formatLocaleDateTime } from '@/utils/format'
import { useStagger } from '@/composables/useStagger'
import UiEmptyState from '@/components/ui/UiEmptyState.vue'
import UiErrorState from '@/components/ui/UiErrorState.vue'
import UiSkeleton from '@/components/ui/UiSkeleton.vue'

const router = useRouter()

const loading = ref(false)
const loadError = ref(false)
const retrying = ref(false)
const favorites = ref<FavoriteItem[]>([])
const total = ref(0)
const currentPage = ref(1)
const pageSize = ref(20)
const activeType = ref<'all' | FavoriteTargetType>('all')

const staggerStyle = useStagger()

const TYPE_LABELS: Record<string, string> = {
  course: '课程',
  chapter: '章节',
  question: '题目',
  featured: '资讯',
  topic: '帖子'
}

const TYPE_COLORS: Record<string, 'primary' | 'success' | 'warning' | 'info' | 'danger'> = {
  course: 'primary',
  chapter: 'success',
  question: 'warning',
  featured: 'info',
  topic: 'danger'
}

function typeLabel(type: string) {
  return TYPE_LABELS[type] || type
}

function typeTagColor(type: string) {
  return TYPE_COLORS[type] || 'info'
}

// 可跳转类型：课程 → 课程中心详情（query 打开），帖子 → 论坛详情；
// 章节/题目/资讯在 web 端无对应详情页，仅展示
function itemPath(item: FavoriteItem): string {
  if (item.target_type === 'course') {
    return `/training/courses?course_id=${item.target_id}`
  }
  if (item.target_type === 'topic') {
    return `/training/forum/${item.target_id}`
  }
  return ''
}

function goItem(item: FavoriteItem) {
  const path = itemPath(item)
  if (path) {
    router.push(path)
  }
}

function handleTabChange() {
  currentPage.value = 1
  loadFavorites()
}

async function loadFavorites() {
  loading.value = true
  loadError.value = false
  try {
    const res = await favoriteApi.list({
      target_type: activeType.value === 'all' ? undefined : activeType.value,
      page: currentPage.value,
      page_size: pageSize.value
    })
    favorites.value = res.favorites || []
    total.value = res.total || 0
  } catch (e) {
    console.error('加载收藏失败:', e)
    loadError.value = true
    favorites.value = []
    total.value = 0
  } finally {
    loading.value = false
  }
}

async function retryLoad() {
  if (retrying.value) return
  retrying.value = true
  try {
    await loadFavorites()
  } finally {
    retrying.value = false
  }
}

async function removeFavorite(item: FavoriteItem) {
  try {
    await ElMessageBox.confirm('确定移除该收藏吗？', '移除收藏', { type: 'warning' })
  } catch {
    return
  }
  try {
    await favoriteApi.remove(item.favorite_id)
    ElMessage.success('已移除')
    // 当前页删空时回退一页（保持至少第 1 页）
    const remain = favorites.value.length - 1
    if (remain === 0 && currentPage.value > 1) {
      currentPage.value -= 1
    }
    loadFavorites()
  } catch (e) {
    console.error('移除收藏失败:', e)
    /* 错误已由拦截器提示 */
  }
}

onMounted(loadFavorites)
</script>

<style scoped>
.favorites-page {
  padding: 20px;
}

.page-header {
  margin-bottom: 12px;
}

.page-header h2 {
  font-size: 22px;
  color: var(--color-text-primary);
}

.favorite-list {
  background: var(--color-bg-card);
  border-radius: 12px;
  box-shadow: 0 2px 12px rgba(0, 0, 0, 0.06);
  min-height: 200px;
}

.favorite-item {
  display: flex;
  align-items: center;
  gap: 14px;
  padding: 14px 20px;
  border-bottom: 1px solid var(--color-border-light);
}

.favorite-item:last-child {
  border-bottom: none;
}

.favorite-item.clickable {
  cursor: pointer;
  transition:
    background var(--duration-tap) var(--ease-default),
    transform var(--duration-tap) var(--ease-default);
}

.favorite-item.clickable:hover {
  background: var(--color-bg-page);
}

.favorite-item.clickable:active {
  background: var(--color-border-light);
  transform: scale(0.995);
}

.item-cover {
  width: 64px;
  height: 48px;
  border-radius: 6px;
  flex-shrink: 0;
  object-fit: cover;
}

.item-cover-fallback {
  display: flex;
  align-items: center;
  justify-content: center;
  background: var(--el-color-primary-light-9, var(--color-primary-50));
  color: var(--el-color-primary, var(--color-primary-500));
  font-size: 20px;
  font-weight: 600;
}

.item-main {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.item-title-row {
  display: flex;
  align-items: center;
  gap: 8px;
  min-width: 0;
}

.item-title {
  font-size: 15px;
  font-weight: 500;
  color: var(--color-text-primary);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.item-time {
  font-size: 12px;
  color: var(--color-text-tertiary);
}

.item-actions {
  flex-shrink: 0;
}

.pagination-wrapper {
  display: flex;
  justify-content: center;
  margin-top: 16px;
}
</style>
