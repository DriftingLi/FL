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
      <UiAsyncSection
        :error="loadError"
        :loading="loading"
        :empty="isEmpty"
        :retrying="retrying"
        error-title="收藏加载失败"
        error-description="网络或服务端异常，可重试"
        @retry="retryLoad"
      >
        <template #skeleton>
          <UiSkeleton variant="list" :count="5" />
        </template>

        <div
          v-for="(item, i) in favorites"
          :key="item.favorite_id"
          class="stagger-in flex items-center gap-3.5 border-b border-line px-5 py-3.5 last:border-b-0"
          :class="
            itemTarget(item)
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
              <UiTag size="small" :tone="typeTagColor(item.target_type)" effect="plain">
                {{ typeLabel(item.target_type) }}
              </UiTag>
              <span class="truncate text-[15px] font-medium text-ink">{{ item.title || `${typeLabel(item.target_type)} #${item.target_id}` }}</span>
            </div>
            <span v-if="item.created_at" class="text-xs text-ink-3">{{ formatLocaleDateTime(item.created_at) }}</span>
          </div>

          <div class="shrink-0" @click.stop>
            <UiButton variant="text" size="small" @click="removeFavorite(item)" class="text-bad">移除</UiButton>
          </div>
        </div>
              <template #empty>
          <UiEmptyState description="暂无收藏" />
                </template>
        </UiAsyncSection>
    </div>

    <div class="mt-4 flex justify-center" v-if="total > pageSize">
      <UiPagination
      v-model:current-page="currentPage"
      :page-size="pageSize"
      :total="total"
      @current-change="handlePageChange"
    />
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { favoriteApi, type FavoriteItem, type FavoriteTargetType } from '@/api/favorite'
import { contentObjectByKey, favoriteTabContentObjects, type ContentObjectTarget } from '@/config/contentObjects'
import { resolveFileUrl } from '@/utils/fileUrl'
import { formatLocaleDateTime } from '@/utils/format'
import { useAsyncPage } from '@/composables/useAsyncPage'
import { useStagger } from '@/composables/useStagger'
import UiAsyncSection from '@/components/ui/UiAsyncSection.vue'
import UiEmptyState from '@/components/ui/UiEmptyState.vue'
import UiSkeleton from '@/components/ui/UiSkeleton.vue'
import UiButton from '@/components/ui/UiButton.vue'
import UiSegmentTabs from '@/components/ui/UiSegmentTabs.vue'
import UiPagination from '@/components/ui/UiPagination.vue'
import { useConfirm } from '@/composables/useConfirm'
import UiTag from '@/components/ui/UiTag.vue'

const router = useRouter()

const favorites = ref<FavoriteItem[]>([])
const activeType = ref<'all' | FavoriteTargetType>('all')

// 三态 + 分页三件套收编（#388）
const {
  loading,
  loadError,
  retrying,
  retry: retryLoad,
  isEmpty,
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
}, { itemsRef: favorites })

const staggerStyle = useStagger()

// 种类 → 称谓/标签色/tab/落点全部派生自内容对象声明表（票 2，#1168）。
// 收藏 tab 的「内容精选刻意不补」裁定（#1132）住在表的 inFavoriteTab 槽（理由见 contentObjects.ts）。
const typeTabOptions = [
  { label: '全部', value: 'all' },
  ...favoriteTabContentObjects().map(o => ({ label: o.label, value: o.favoriteTargetType }))
]

function typeLabel(type: string) {
  return contentObjectByKey(type)?.label || type
}

function typeTagColor(type: string) {
  return contentObjectByKey(type)?.tone || 'info'
}

// 落点装配在表里（与 SearchPage 同一事实源）；章节的所属课程由 FavoriteDTO.course_id 给出（#1089），
// 缺失即无落点 → 条目不可点（不猜、不乱跳）。
function itemTarget(item: FavoriteItem): ContentObjectTarget | null {
  const o = contentObjectByKey(item.target_type)
  return o ? o.to({ id: item.target_id, parentId: item.course_id }) : null
}

function goItem(item: FavoriteItem) {
  const target = itemTarget(item)
  if (target) {
    router.push(target)
  }
}

function handleTabChange() {
  currentPage.value = 1
  loadFavorites()
}

async function removeFavorite(item: FavoriteItem) {
  try {
    await useConfirm().confirmDanger('确定移除该收藏吗？', '移除收藏', { type: 'warning' })
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
