<template>
  <div class="mx-auto max-w-[960px] px-4 pb-10">
    <div class="forum-header mb-4 flex items-start justify-between gap-4 max-md:flex-col">
      <h1 class="m-0 mb-1.5 text-2xl font-semibold text-ink">学员论坛</h1>
      <UiButton variant="primary" v-if="mainTab === 'question'" size="large" :icon="EditPen" @click="goAsk">
        我要提问
      </UiButton>
      <UiButton variant="primary" v-else size="large" :icon="EditPen" @click="openCreateDialog">
        发布新帖
      </UiButton>
    </div>

    <!-- 每日打卡入口（ADR-0028：打卡已迁独立页 /training/check-in，论坛只留跳转） -->
    <UiCard variant="interactive" padding="base" class="mb-3 flex items-center justify-between gap-3" @click="goCheckIn">
      <div class="flex items-center gap-2 text-[13px] text-ink">
        <el-icon class="text-base text-ui-500"><Calendar /></el-icon>
        <span>每日打卡赚积分</span>
      </div>
      <UiButton variant="text" size="small">去看看</UiButton>
    </UiCard>

    <!-- 一级 Tab（解耦后的版本）：
         讨论 / 问答 看的是"内容类别"（含排序/求助筛选），点击我的帖子/我的回复/浏览记录只是
         把过滤维度从"看哪一类"换成了"看谁的"——原本同时存在的两套 Tab 会互相覆盖消失。
         这里把"看谁的"下沉成"我的"的二级 Tab，正交的三个维度就此拆开。 -->
    <UiSegmentTabs
      v-model="mainTab"
      :options="[
        { label: '讨论', value: 'discussion' },
        { label: '问答', value: 'question' },
        { label: '备考经验', value: 'experience' },
        { label: '我的', value: 'mine' }
      ]"
      class="mb-3"
    />

    <!-- 排序 / 求助筛选（讨论、问答、经验）：精选筛选三 Tab 通用（#742），问答额外叠求助/已解决 -->
    <div v-if="mainTab !== 'mine'" class="mb-3 flex flex-wrap items-center justify-between gap-3">
      <div class="flex flex-wrap items-center gap-2">
        <div v-if="mainTab === 'question'" class="solved-filter">
          <UiSegmentTabs
            :model-value="solvedFilter"
            :options="[
              { label: '全部', value: 'all' },
              { label: '求助', value: 'unsolved' },
              { label: '已解决', value: 'solved' }
            ]"
            @update:model-value="(v: string) => { solvedFilter = v as 'all' | 'unsolved' | 'solved'; handleSolvedChange() }"
          />
        </div>
        <UiSegmentTabs
          :model-value="featuredFilter"
          :options="[
            { label: '全部帖', value: '' },
            { label: '★ 精选', value: 'true' }
          ]"
          @update:model-value="(v: string) => { featuredFilter = v as '' | 'true'; handleFeaturedChange() }"
        />
      </div>
      <div class="ml-auto flex items-center gap-2">
        <UiSegmentTabs
          :model-value="topicSort"
          :options="[
            { label: '最新', value: 'latest' },
            { label: '热门', value: 'hot' }
          ]"
          @update:model-value="(v: string) => { topicSort = v as 'latest' | 'hot'; handleSortChange() }"
        />
        <UiButton size="small" :icon="topicOrder === 'asc' ? ArrowUp : ArrowDown" @click="toggleTopicOrder">
          {{ topicOrder === 'asc' ? '正序' : '逆序' }}
        </UiButton>
      </div>
    </div>

    <!-- 我的 二级 Tab（无排序，天然跨类别） -->
    <UiSegmentTabs
      v-else
      :model-value="mineTab"
      :options="[
        { label: '我的帖子', value: 'my-topics' },
        { label: '我的回复', value: 'my-replies' },
        { label: '赞过', value: 'my-liked' },
        { label: '围观', value: 'my-observed' },
        { label: '浏览记录', value: 'history' }
      ]"
      @update:model-value="(v: string) => { mineTab = v as MineTab; handleMineTabChange() }"
      class="mb-3"
    />

    <!-- 我的帖子 / 赞过 / 围观 / 浏览记录共用主题列表渲染（仅我的回复走独立分支） -->
    <!-- 我的回复列表（条目带主题标题回填，点击跳对应帖子） -->
    <div v-if="showReplies" class="min-h-[300px] rounded-card bg-panel shadow-card">
      <UiErrorState
        v-if="loadError"
        title="回复加载失败"
        description="网络或服务端异常，可重试"
        :retrying="retrying"
        @retry="retryLoad"
      />

      <UiSkeleton v-else-if="loading" variant="list" :count="5" />

      <template v-else-if="myReplies.length > 0">
        <div
          v-for="(reply, i) in myReplies"
          :key="reply.id"
          class="stagger-in flex cursor-pointer gap-3.5 border-b border-line px-5 py-[18px] transition-[background,transform] duration-[var(--duration-tap)] ease-[var(--ease-default)] hover:bg-canvas active:scale-[0.995] active:bg-line last:border-b-0 max-md:px-3.5 max-md:py-3.5"
          :style="staggerStyle(i)"
          @click="goDetail(reply.topic_id)"
        >
          <div class="min-w-0 flex-1">
            <div class="flex flex-wrap items-center gap-2">
              <UiTag size="small" tone="info">回复</UiTag>
              <h3 class="m-0 truncate text-base font-semibold text-ink">{{ reply.topic_title || '原帖已删除' }}</h3>
            </div>
            <p class="mt-1.5 mb-2 line-clamp-2 text-[13px] text-ink-2">{{ reply.content }}</p>
            <div class="flex items-center gap-1.5 text-xs text-ink-3">
              <span>{{ formatRelativeTime(reply.created_at) }}</span>
            </div>
          </div>
        </div>
      </template>
      <UiEmptyState v-else description="暂无回复" />
    </div>

    <div v-else class="min-h-[300px] rounded-card bg-panel shadow-card">
      <UiErrorState
        v-if="loadError"
        title="帖子加载失败"
        description="网络或服务端异常，可重试"
        :retrying="retrying"
        @retry="retryLoad"
      />

      <UiSkeleton v-else-if="loading" variant="list" :count="5" />

      <template v-else-if="topics.length > 0">
        <div
          v-for="(topic, i) in topics"
          :key="topic.id"
          class="stagger-in flex cursor-pointer gap-3.5 border-b border-line px-5 py-[18px] transition-[background,transform] duration-[var(--duration-tap)] ease-[var(--ease-default)] hover:bg-canvas active:scale-[0.995] active:bg-line last:border-b-0 max-md:px-3.5 max-md:py-3.5"
          :style="staggerStyle(i)"
          @click="goDetail(topic.id)"
        >
          <div>
            <el-avatar :size="42" :src="topic.author.avatar_url || undefined">
              {{ authorLetter(topic.author) }}
            </el-avatar>
          </div>
          <div class="min-w-0 flex-1">
            <div class="flex flex-wrap items-center gap-2">
              <template v-if="topic.category === 'question'">
                <UiTag v-if="topic.accepted_reply_id || topic.solved_at" size="small" tone="success" effect="dark" class="font-semibold">✓ 已解决</UiTag>
                <UiTag v-else size="small" tone="info" effect="plain" class="bg-canvas text-ink-2 border-[var(--color-border-dark)]">求助</UiTag>
              </template>
              <UiTag v-else-if="topic.chapter_id" size="small" tone="warning">
                {{ topic.chapter_title || '章节讨论' }}
              </UiTag>
              <UiTag v-else size="small" tone="info">综合</UiTag>
              <UiTag v-if="topic.is_featured" size="small" effect="dark" class="font-semibold">★ 精选</UiTag>
              <h3 class="m-0 truncate text-base font-semibold text-ink">{{ topic.title }}</h3>
            </div>
            <p class="mt-1.5 mb-2 line-clamp-2 text-[13px] text-ink-2">{{ topic.content }}</p>
            <div class="flex items-center gap-1.5 text-xs text-ink-3">
              <span>{{ displayName(topic.author) }}</span>
              <span class="text-[var(--color-border-dark)]">·</span>
              <span>{{ formatRelativeTime(topic.created_at) }}</span>
              <span class="ml-auto flex items-center gap-1">
                <span v-if="topic.images && topic.images.length > 0" class="mr-2.5 inline-flex items-center gap-0.5 text-warn">
                  <el-icon><Picture /></el-icon>
                  {{ topic.images.length }}
                </span>
                <el-icon><View /></el-icon>
                {{ topic.view_count }}
                <span class="ml-2.5 inline-flex items-center gap-0.5 text-bad">
                  <span class="text-xs leading-none">{{ topic.liked_by_me ? '♥' : '♡' }}</span>
                  {{ topic.likes_count || 0 }}
                </span>
                <el-icon class="ml-2.5"><ChatDotRound /></el-icon>
                {{ topic.reply_count }}
              </span>
            </div>
          </div>
        </div>
      </template>
      <UiEmptyState v-else :description="emptyDescription" :action-text="mainTab === 'question' ? '我要提问' : undefined" @action="goAsk" />
    </div>

    <div class="mt-5 flex justify-center" v-if="total > pageSize">
      <UiPagination
      v-model:current-page="currentPage"
      :page-size="pageSize"
      :total="total"
      @current-change="handlePageChange"
    />
    </div>

    <!-- 发布入口（ADR-0040）：经验 Tab 已退为只读策展流——表单默认落「讨论」，且只提供
         讨论/问答两个意图（即便从经验 Tab 打开，表单也会把历史 experience 归一为 discussion）。 -->
    <UiDialog
      v-model="createDialogVisible"
      title="发布新帖"
      :icon="EditPen"
      width="640px"
      confirm-text="发布"
      :confirm-loading="postForm?.submitting"
      @confirm="postForm?.submit()"
    >
      <ForumPostForm
        ref="postForm"
        category="discussion"
        :categories="['discussion', 'question']"
        @success="onTopicCreated"
      />
    </UiDialog>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, watch, nextTick, onMounted } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { EditPen, View, ChatDotRound, Picture, Calendar, ArrowUp, ArrowDown } from '@element-plus/icons-vue'
import { forumApi, forumTabQuery, type ForumCategory, type ForumTopicItem, type MyReplyItem } from '@/api/forum'
import { formatRelativeTime } from '@/utils/format'
import { displayName, authorLetter } from '@/utils/forumDisplay'
import ForumPostForm from '@/components/student/ForumPostForm.vue'
import { useAsyncPage } from '@/composables/useAsyncPage'
import { useForumSort } from '@/composables/useForumSort'
import { useStagger } from '@/composables/useStagger'
import UiEmptyState from '@/components/ui/UiEmptyState.vue'
import UiErrorState from '@/components/ui/UiErrorState.vue'
import UiSkeleton from '@/components/ui/UiSkeleton.vue'
import UiButton from '@/components/ui/UiButton.vue'
import UiSegmentTabs from '@/components/ui/UiSegmentTabs.vue'
import UiCard from '@/components/ui/UiCard.vue'
import UiDialog from '@/components/ui/UiDialog.vue'
import UiPagination from '@/components/ui/UiPagination.vue'
import UiTag from '@/components/ui/UiTag.vue'

const router = useRouter()
const route = useRoute()

const staggerStyle = useStagger()
const topics = ref<ForumTopicItem[]>([])
const myReplies = ref<MyReplyItem[]>([])

// ===== 一级 Tab（讨论 / 问答 / 备考经验 / 我的）=====
// 选哪一片内容看。"我的"是个人视图（无排序、跨类别），原「模式」轴整体下沉为它的二级 Tab。
type MainTab = ForumCategory | 'mine'
const mainTab = ref<MainTab>('discussion')

// ===== 我的 二级 Tab（我的帖子 / 我的回复 / 赞过 / 围观 / 浏览记录，#701）=====
type MineTab = 'my-topics' | 'my-replies' | 'my-liked' | 'my-observed' | 'history'
const mineTab = ref<MineTab>('my-topics')

// ===== 排序双轴收编（#389）：切维度回默认降序（最新/最热优先）=====
const { sort: topicSort, order: topicOrder, flipOrder, resetOrder } = useForumSort('desc')

// ===== 求助/已解决筛选（#367）：仅问答 Tab 的筛选轴 =====
type SolvedFilter = 'all' | 'solved' | 'unsolved'
const solvedFilter = ref<SolvedFilter>('all')

// ===== 精选筛选（#742）：三 Tab 通用轴，'' = 全部帖 / 'true' = 仅精选 =====
const featuredFilter = ref<'' | 'true'>('')

function handleSolvedChange() {
  currentPage.value = 1
  loadTopics()
}

function handleFeaturedChange() {
  currentPage.value = 1
  loadTopics()
}

// 分页与滚动位置按一级 Tab 各存一份：切走再切回来仍停在原来的位置。
const pageByTab = ref<Record<MainTab, number>>({ discussion: 1, question: 1, experience: 1, mine: 1 })
const scrollByTab: Record<MainTab, number> = { discussion: 0, question: 0, experience: 0, mine: 0 }

const currentPage = computed({
  get: () => pageByTab.value[mainTab.value],
  set: (v: number) => {
    pageByTab.value[mainTab.value] = v
  }
})

// 内容分支：仅我的回复走独立分支，其余（我的帖子/赞过/围观/浏览记录）同走主题列表分支。
const showReplies = computed(() => mainTab.value === 'mine' && mineTab.value === 'my-replies')

// 问答 Tab 空态升级为引导（#365 提问入口就位后）：指向"我要提问"
// 我的 Tab 各视图空态文案（#701）：与移动端个人动态页口径对齐
const emptyDescription = computed(() => {
  if (mainTab.value === 'question') return '还没有人提问，来发第一个提问吧'
  // 经验区是管理端认定的只读策展流（ADR-0040）：不再有「发第一篇」入口，文案不引导发布
  if (mainTab.value === 'experience') return '还没有备考经验帖'
  if (mainTab.value === 'mine') {
    switch (mineTab.value) {
      case 'my-replies':
        return '还没有回复过帖子'
      case 'my-liked':
        return '还没有点赞过帖子'
      case 'my-observed':
        return '还没有围观过帖子'
      case 'history':
        return '还没有浏览记录'
      default:
        return '你还没有发布过帖子，去讨论区发第一帖吧'
    }
  }
  return '还没有帖子，来发第一帖吧'
})

watch(mainTab, async (next, prev) => {
  scrollByTab[prev] = window.scrollY
  // 切换一级 Tab 时重置 solved 筛选为全部，避免讨论筛漏到问答
  if (next !== 'question' && solvedFilter.value !== 'all') {
    solvedFilter.value = 'all'
  }
  // 精选筛选跨 Tab 不延续（#742）：切 Tab 回到全部帖
  if (featuredFilter.value !== '') {
    featuredFilter.value = ''
  }
  await loadTopics()
  await nextTick()
  window.scrollTo?.({ top: scrollByTab[next] })
})

function handleMineTabChange() {
  // 二级 Tab 切换：重置分页并刷新。"我的"内五个视图共用 pageByTab.mine，
  // 切换回到同一页 1，避免"上次切走停留在第 5 页"这种残留。
  currentPage.value = 1
  loadTopics()
}

function handleSortChange() {
  currentPage.value = 1
  // 切换排序维度时重置为降序（最新/最热优先）
  resetOrder()
  loadTopics()
}

function toggleTopicOrder() {
  flipOrder()
  loadTopics()
}

const createDialogVisible = ref(false)
// 发帖表单体（#389）：字段/校验/提交在 ForumPostForm，本页只留壳与发布后刷新
const postForm = ref<InstanceType<typeof ForumPostForm> | null>(null)

// 三态 + 分页收编（#388）：页码由按一级 Tab 分片的 currentPage（可写 computed）持有，经 pageRef 注入
const {
  loading,
  loadError,
  retrying,
  retry: retryLoad,
  pageSize,
  total,
  run: loadTopics,
  handlePageChange
} = useAsyncPage(loadTopicsOnce, {
  pageRef: currentPage,
  defaultPageSize: 10,
  // 论坛不受证件过滤（CONTEXT.md「当前证件」），不随切换重装/重置页码（#604 opt-out）
  credentialScoped: false
})

async function loadTopicsOnce() {
  const params = { page: currentPage.value, page_size: pageSize.value }
  const activeMain = mainTab.value

  if (activeMain === 'mine') {
    const activeMine = mineTab.value
    if (activeMine === 'my-replies') {
      const res = await forumApi.getMyReplies(params)
      myReplies.value = res.replies || []
      total.value = res.total || 0
      return
    }
    // 我的帖子 / 赞过 / 围观 / 浏览记录四视图同走主题列表渲染（#701：响应逐字沿用 my-topics 形态）
    const fetcher =
      activeMine === 'my-liked'
        ? forumApi.getMyLikedTopics
        : activeMine === 'my-observed'
          ? forumApi.getMyObservedTopics
          : activeMine === 'history'
            ? forumApi.getMyViewHistory
            : forumApi.getMyTopics
    const res = await fetcher(params)
    topics.value = res.topics || []
    total.value = res.total || 0
    return
  }

  // activeMain: 'discussion' | 'question' | 'experience'（#722 备考经验进场）
  // 查询参数交给 forumTabQuery 统一翻译（与端共用同一份映射）。
  // 关键是讨论 Tab 必须带 category=discussion：后端 scope=general 的定义就是
  // chapter_id IS NULL，而问答帖与经验帖的 chapter_id 同为 NULL，漏 category 会让它们灌进讨论列表。
  const query = {
    ...forumTabQuery(activeMain),
    sort: topicSort.value,
    order: topicOrder.value,
    ...params
  } as Record<string, unknown>
  // 已解决/求助仅对问答生效（#367 单一筛选轴）
  if (activeMain === 'question' && solvedFilter.value !== 'all') {
    ;(query as { solved?: string }).solved = solvedFilter.value
  }
  // 精选筛选（#742）：三 Tab 通用，'' 表示不传（不过滤）
  if (featuredFilter.value !== '') {
    ;(query as { featured?: string }).featured = featuredFilter.value
  }
  const res = await forumApi.listTopics(query as Parameters<typeof forumApi.listTopics>[0])
  topics.value = res.topics || []
  total.value = res.total || 0
}

function openCreateDialog() {
  postForm.value?.reset()
  createDialogVisible.value = true
}

// 发布成功（表单体 #389）：关壳 + 列表刷新语义留在本页
async function onTopicCreated() {
  createDialogVisible.value = false
  // 停在问答 Tab 时新发的讨论帖会被该 Tab 的 category 过滤掉，用户会看到「发布成功」
  // 但列表里什么都没有；切回讨论 Tab（watch 会负责重新拉取）让它立刻可见。
  if (mainTab.value !== 'discussion') {
    pageByTab.value.discussion = 1
    mainTab.value = 'discussion'
    return
  }
  currentPage.value = 1
  await loadTopics()
}

function goDetail(id: number) {
  router.push({ name: 'ForumDetail', params: { topicId: String(id) } })
}

function goAsk() {
  router.push({ name: 'ForumAsk' })
}

// 打卡入口跳独立页（ADR-0028：打卡从论坛剥离为独立激励面）
function goCheckIn() {
  router.push({ name: 'CheckIn' })
}

onMounted(() => {
  // 允许从路由 ?tab= 跳转进指定 Tab（与 nav 链接/通知深链共用一套语义）
  if (route.query.tab === 'question') {
    mainTab.value = 'question'
  } else if (route.query.tab === 'mine') {
    mainTab.value = 'mine'
  }
  loadTopics()
})

watch(
  () => route.query.tab,
  (tab) => {
    if (tab === 'question' && mainTab.value !== 'question') {
      mainTab.value = 'question'
    } else if (tab === 'mine' && mainTab.value !== 'mine') {
      mainTab.value = 'mine'
    } else if (tab === 'discussion' && mainTab.value !== 'discussion') {
      mainTab.value = 'discussion'
    }
  }
)
</script>