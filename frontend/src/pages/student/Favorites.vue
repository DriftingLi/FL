<template>
  <div class="p-5">
    <div class="mb-3">
      <h2 class="text-[22px] text-ink">我的收藏</h2>
    </div>

    <!-- #511：分类 tab 统一分段控件 -->
    <UiSegmentTabs
      :model-value="activeType"
      :options="typeTabOptions"
      @update:model-value="(v: string) => { activeType = v as 'all' | FavoriteTargetType; handleTabChange() }"
      class="mb-3"
    />

    <div class="min-h-[200px] rounded-card bg-panel shadow-card">
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
          class="stagger-in flex items-center gap-3.5 border-b border-line px-5 py-3.5 last:border-b-0"
          :class="
            itemPath(item)
              ? 'cursor-pointer transition-[background,transform] duration-[var(--duration-tap)] ease-[var(--ease-default)] hover:bg-canvas active:scale-[0.995] active:bg-line'
              : ''
          "
          :style="staggerStyle(i)"
          @click="goItem(item)"
        >
          <el-image
            v-if="item.cover"
            :src="resolveFileUrl(item.cover)"
            fit="cover"
            class="h-12 w-16 shrink-0 rounded-[6px] object-cover"
          >
            <template #error>
              <div class="flex h-12 w-16 shrink-0 items-center justify-center rounded-[6px] bg-ui-50 text-xl font-semibold text-ui-500">
                {{ typeLabel(item.target_type).charAt(0) }}
              </div>
            </template>
          </el-image>
          <div v-else class="flex h-12 w-16 shrink-0 items-center justify-center rounded-[6px] bg-ui-50 text-xl font-semibold text-ui-500">
            {{ typeLabel(item.target_type).charAt(0) }}
          </div>

          <div class="flex min-w-0 flex-1 flex-col gap-1.5">
            <div class="flex min-w-0 items-center gap-2">
              <el-tag size="small" :type="typeTagColor(item.target_type)" effect="plain">
                {{ typeLabel(item.target_type) }}
              </el-tag>
              <span class="truncate text-[15px] font-medium text-ink">{{ item.title || `${typeLabel(item.target_type)} #${item.target_id}` }}</span>
            </div>
            <span v-if="item.created_at" class="text-xs text-ink-3">{{ formatLocaleDateTime(item.created_at) }}</span>
          </div>

          <div class="shrink-0" @click.stop>
            <UiButton variant="text" size="small" @click="removeFavorite(item)" class="text-bad">移除</UiButton>
          </div>
        </div>
      </template>
      <UiEmptyState v-else description="暂无收藏" />
    </div>

    <div class="mt-4 flex justify-center" v-if="total > pageSize">
      <el-pagination
        v-model:current-page="currentPage"
        :page-size="pageSize"
        :total="total"
        layout="total, prev, pager, next"
        @current-change="handlePageChange"
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
import { useAsyncPage } from '@/composables/useAsyncPage'
import { useStagger } from '@/composables/useStagger'
import UiEmptyState from '@/components/ui/UiEmptyState.vue'
import UiErrorState from '@/components/ui/UiErrorState.vue'
import UiSkeleton from '@/components/ui/UiSkeleton.vue'
import UiButton from '@/components/ui/UiButton.vue'
import UiSegmentTabs from '@/components/ui/UiSegmentTabs.vue'

const router = useRouter()

const favorites = ref<FavoriteItem[]>([])
const activeType = ref<'all' | FavoriteTargetType>('all')

// 三态 + 分页三件套收编（#388）
const {
  loading,
  loadError,
  retrying,
  retry: retryLoad,
  page: currentPage,
  pageSize,
  total,
  run: loadFavorites,
  handlePageChange
} = useAsyncPage(async () => {
  const res = await favoriteApi.list({
    target_type: activeType.value === 'all' ? undefined : activeType.value,
    page: currentPage.value,
    page_size: pageSize.value
  })
  favorites.value = res.favorites || []
  total.value = res.total || 0
})

const staggerStyle = useStagger()

const TYPE_LABELS: Record<string, string> = {
  course: '课程',
  chapter: '章节',
  question: '题目',
  featured: '资讯',
  topic: '帖子'
}

// #511：UiSegmentTabs 分类选项（顶部 tab 轴：全部/课程/题目/帖子）
const typeTabOptions = [
  { label: '全部', value: 'all' },
  { label: '课程', value: 'course' },
  { label: '题目', value: 'question' },
  { label: '帖子', value: 'topic' }
]

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
