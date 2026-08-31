<template>
  <div class="mx-auto max-w-[960px] px-4 pb-10">
    <div class="forum-header mb-4 flex items-start justify-between gap-4 max-md:flex-col">
      <h1 class="m-0 mb-1.5 text-2xl font-semibold text-ink">学员论坛</h1>
      <UiButton variant="primary" v-if="mode === 'all' && categoryTab === 'question'" size="large" :icon="EditPen" @click="goAsk">
        我要提问
      </UiButton>
      <UiButton variant="primary" v-else size="large" :icon="EditPen" @click="openCreateDialog">
        发布新帖
      </UiButton>
    </div>

    <!-- 每日打卡卡片（spec #268 A1） -->
    <UiCard variant="interactive" padding="base" class="mb-3 flex items-center justify-between gap-3" @click="openCheckIn('calendar')">
      <div class="flex items-center gap-2 text-[13px] text-ink">
        <el-icon class="text-base text-ui-500"><Calendar /></el-icon>
        <span>
          <template v-if="checkInTodayChecked">今日已打卡</template>
          <template v-else>今日未打卡</template>
          <span v-if="checkInTotal > 0" class="ml-1.5 text-xs text-ink-3">｜已连续 {{ checkInStreak }} 天｜累计 {{ checkInTotal }} 天</span>
        </span>
      </div>
      <div class="flex items-center gap-1" @click.stop>
        <UiButton variant="primary" size="small" :loading="checkInChecking" :disabled="checkInTodayChecked" @click="doCheckIn">
          {{ checkInTodayChecked ? '今日已打卡' : '立即打卡' }}
        </UiButton>
        <UiButton variant="text" size="small" @click="openCheckIn('rank')">排行榜</UiButton>
      </div>
    </UiCard>

    <!-- 类别分流（#364）：讨论 / 问答。与下面的 mode 轴正交——category 管"看哪类"，mode 管"看谁的"。
         只在浏览态显示：我的帖子/我的回复/浏览记录是个人视图，天然跨类别（后端 my-topics 也无 category 维度）。 -->
    <el-radio-group v-if="mode === 'all'" v-model="categoryTab" class="forum-category mb-3">
      <el-radio-button value="discussion">讨论</el-radio-button>
      <el-radio-button value="question">问答</el-radio-button>
    </el-radio-group>

    <!-- 求助/已解决筛选（#367）：仅问答 Tab 的唯一筛选轴，不加章节筛选 -->
    <div v-if="mode === 'all' && categoryTab === 'question'" class="solved-filter mb-3">
      <el-radio-group v-model="solvedFilter" size="small" @change="handleSolvedChange">
        <el-radio-button value="all">全部</el-radio-button>
        <el-radio-button value="unsolved">求助</el-radio-button>
        <el-radio-button value="solved">已解决</el-radio-button>
      </el-radio-group>
    </div>

    <div class="mb-3 flex items-center justify-between gap-3">
      <el-radio-group v-model="mode" class="forum-mode" @change="handleModeChange">
        <el-radio-button value="all">全部</el-radio-button>
        <el-radio-button value="my-topics">我的帖子</el-radio-button>
        <el-radio-button value="my-replies">我的回复</el-radio-button>
        <el-radio-button value="history">浏览记录</el-radio-button>
      </el-radio-group>
      <el-radio-group v-if="mode !== 'my-replies' && mode !== 'history'" v-model="topicSort" size="small" @change="handleSortChange">
        <el-radio-button value="latest">最新</el-radio-button>
        <el-radio-button value="hot">热门</el-radio-button>
      </el-radio-group>
      <UiButton v-if="mode !== 'my-replies' && mode !== 'history'" size="small" :icon="topicOrder==='asc'? ArrowUp : ArrowDown" @click="toggleTopicOrder">{{ topicOrder==='asc' ? '正序' : '逆序' }}</UiButton>
    </div>

    <CheckInDialog v-model="checkInDialogVisible" :initial-tab="checkInTab" @checked="onCheckInChecked" />

    <!-- 浏览记录（卡片分组，选型 b） -->
    <div v-if="mode === 'history'">
      <ForumHistoryPanel :items="historyItems" @select="handleHistorySelect" @remove="handleHistoryRemove" @clear="handleHistoryClear" />
    </div>

    <!-- 我的回复列表（条目带主题标题回填，点击跳对应帖子） -->
    <div v-else-if="mode === 'my-replies'" class="min-h-[300px] rounded-card bg-panel shadow-card">
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
              <el-tag size="small" type="info">回复</el-tag>
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
                <el-tag v-if="topic.accepted_reply_id || topic.solved_at" size="small" type="success" effect="dark" class="font-semibold">✓ 已解决</el-tag>
                <el-tag v-else size="small" type="info" effect="plain" class="bg-canvas text-ink-2 border-[var(--color-border-dark)]">求助</el-tag>
              </template>
              <el-tag v-else-if="topic.chapter_id" size="small" type="warning">
                {{ topic.chapter_title || '章节讨论' }}
              </el-tag>
              <el-tag v-else size="small" type="info">综合</el-tag>
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
      <UiEmptyState v-else :description="emptyDescription" :action-text="mode === 'all' && categoryTab === 'question' ? '我要提问' : undefined" @action="goAsk" />
    </div>

    <div class="mt-5 flex justify-center" v-if="mode !== 'history' && total > pageSize">
      <el-pagination
        v-model:current-page="currentPage"
        :page-size="pageSize"
        :total="total"
        layout="total, prev, pager, next"
        @current-change="handlePageChange"
      />
    </div>

    <el-dialog v-model="createDialogVisible" title="发布新帖" width="640px">
      <ForumPostForm ref="postForm" category="discussion" @success="onTopicCreated" />
      <template #footer>
        <UiButton @click="createDialogVisible = false">取消</UiButton>
        <UiButton variant="primary" :loading="postForm?.submitting" @click="postForm?.submit()">发布</UiButton>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, watch, nextTick, onMounted } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { ElMessage } from 'element-plus'
import { EditPen, View, ChatDotRound, Picture, Calendar, ArrowUp, ArrowDown } from '@element-plus/icons-vue'
import { forumApi, forumTabQuery, type ForumCategory, type ForumTopicItem, type MyReplyItem } from '@/api/forum'
import { formatRelativeTime } from '@/utils/format'
import { displayName, authorLetter } from '@/utils/forumDisplay'
import ForumPostForm from '@/components/student/ForumPostForm.vue'
import CheckInDialog from '@/components/student/CheckInDialog.vue'
import ForumHistoryPanel from '@/components/student/ForumHistoryPanel.vue'
import { loadHistory, removeHistoryItem, clearHistory } from '@/utils/forumHistory'
import type { ForumHistoryItem } from '@/utils/forumHistory'
import { useAuthStore } from '@/stores/auth'
import { useAsyncPage } from '@/composables/useAsyncPage'
import { useForumSort } from '@/composables/useForumSort'
import { useStagger } from '@/composables/useStagger'
import UiEmptyState from '@/components/ui/UiEmptyState.vue'
import UiErrorState from '@/components/ui/UiErrorState.vue'
import UiSkeleton from '@/components/ui/UiSkeleton.vue'
import UiButton from '@/components/ui/UiButton.vue'
import UiCard from '@/components/ui/UiCard.vue'

const router = useRouter()
const route = useRoute()
const authStore = useAuthStore()

const staggerStyle = useStagger()
const topics = ref<ForumTopicItem[]>([])
const myReplies = ref<MyReplyItem[]>([])

// 列表模式：全部 / 我的帖子 / 我的回复 / 浏览记录
type ForumMode = 'all' | 'my-topics' | 'my-replies' | 'history'
const mode = ref<ForumMode>('all')

// 排序双轴收编（#389）：切维度回默认降序（最新/最热优先）
const { sort: topicSort, order: topicOrder, flipOrder, resetOrder } = useForumSort('desc')
const historyItems = ref<ForumHistoryItem[]>([])

// ===== 类别分流（#364）=====
// category 判"看哪一类帖"，mode 判"看谁的帖"，两个轴正交。
const categoryTab = ref<ForumCategory>('discussion')

// ===== 求助/已解决筛选（#367）：仅问答 Tab 的唯一筛选轴 =====
type SolvedFilter = 'all' | 'solved' | 'unsolved'
const solvedFilter = ref<SolvedFilter>('all')

function handleSolvedChange() {
  currentPage.value = 1
  loadTopics()
}

// 分页与滚动位置按类别各存一份：切走再切回来仍停在原来的位置。
// currentPage 做成"按当前 Tab 读写的可写 computed"，这样既有代码里的
// `currentPage.value = 1` 一行都不用改，也不存在两处状态手工同步漏一处的风险。
const pageByCategory = ref<Record<ForumCategory, number>>({ discussion: 1, question: 1 })
const scrollByCategory: Record<ForumCategory, number> = { discussion: 0, question: 0 }

const currentPage = computed({
  get: () => pageByCategory.value[categoryTab.value],
  set: (v: number) => {
    pageByCategory.value[categoryTab.value] = v
  }
})

// 问答 Tab 空态升级为引导（#365 提问入口就位后）：指向“我要提问”
const emptyDescription = computed(() =>
  mode.value === 'all' && categoryTab.value === 'question'
    ? '还没有人提问，来发第一个提问吧'
    : '还没有帖子，来发第一帖吧'
)

watch(categoryTab, async (next, prev) => {
  scrollByCategory[prev] = window.scrollY
  // 切换类别时重置 solved 筛选为全部，避免讨论筛漏到问答
  if (next !== 'question' && solvedFilter.value !== 'all') {
    solvedFilter.value = 'all'
  }
  await loadTopics()
  await nextTick()
  window.scrollTo?.({ top: scrollByCategory[next] })
})

function handleModeChange() {
  currentPage.value = 1
  if (mode.value === 'history') {
    loadHistoryItems()
  } else {
    loadTopics()
  }
}

function loadHistoryItems() {
  historyItems.value = loadHistory(authStore.userInfo?.user_id)
}

function handleHistorySelect(id: number) {
  const found = historyItems.value.find((h) => h.id === id)
  if (found?.deleted) {
    ElMessage.warning('原帖已删除')
    return
  }
  goDetail(id)
}

function handleHistoryRemove(id: number) {
  historyItems.value = removeHistoryItem(id, authStore.userInfo?.user_id)
}

function handleHistoryClear() {
  historyItems.value = clearHistory(authStore.userInfo?.user_id)
  ElMessage.success('已清空浏览记录')
}

function handleSortChange() {
  currentPage.value = 1
  // 切换排序维度时重置为降序（最新/最热优先）
  resetOrder()
  loadTopics()
}

function toggleTopicOrder(){
  flipOrder()
  loadTopics()
}

const createDialogVisible = ref(false)
// 发帖表单体（#389）：字段/校验/提交在 ForumPostForm，本页只留壳与发布后刷新
const postForm = ref<InstanceType<typeof ForumPostForm> | null>(null)

// 三态 + 分页收编（#388）：页码由按类别分片的 currentPage（可写 computed）持有，经 pageRef 注入
const {
  loading,
  loadError,
  retrying,
  retry: retryLoad,
  pageSize,
  total,
  run: loadTopics,
  handlePageChange
} = useAsyncPage(loadTopicsOnce, { pageRef: currentPage, defaultPageSize: 10 })

async function loadTopicsOnce() {
  const params = { page: currentPage.value, page_size: pageSize.value }
  if (mode.value === 'my-replies') {
    const res = await forumApi.getMyReplies(params)
    myReplies.value = res.replies || []
    total.value = res.total || 0
  } else if (mode.value === 'my-topics') {
    const res = await forumApi.getMyTopics(params)
    topics.value = res.topics || []
    total.value = res.total || 0
  } else {
    // 查询参数交给 forumTabQuery 统一翻译（与端共用同一份映射）。
    // 关键是讨论 Tab 必须带 category=discussion：后端 scope=general 的定义就是
    // chapter_id IS NULL，而问答帖的 chapter_id 同为 NULL，漏 category 会让问答帖整片灌进讨论列表。
    const query = {
      ...forumTabQuery(categoryTab.value),
      sort: topicSort.value,
      order: topicOrder.value,
      ...params
    } as Record<string, unknown>
    // 已解决/求助仅对问答生效（#367 单一筛选轴）
    if (categoryTab.value === 'question' && solvedFilter.value !== 'all') {
      ;(query as { solved?: string }).solved = solvedFilter.value
    }
    const res = await forumApi.listTopics(query as Parameters<typeof forumApi.listTopics>[0])
    topics.value = res.topics || []
    total.value = res.total || 0
  }
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
  if (mode.value === 'all' && categoryTab.value !== 'discussion') {
    pageByCategory.value.discussion = 1
    categoryTab.value = 'discussion'
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

// ===== 打卡（spec #268 A1-A5）=====
const checkInStreak = ref(0)
const checkInTotal = ref(0)
const checkInTodayChecked = ref(false)
const checkInChecking = ref(false)
const checkInDialogVisible = ref(false)
const checkInTab = ref<'calendar' | 'rank'>('calendar')

async function loadCheckInStatus() {
  try {
    const now = new Date()
    const res = await forumApi.getCheckInCalendar({ year: now.getFullYear(), month: now.getMonth() + 1 })
    checkInStreak.value = res.streak
    checkInTotal.value = res.total
    checkInTodayChecked.value = res.today_checked
  } catch (e) {
    console.error('加载打卡状态失败:', e)
  }
}

function openCheckIn(tab: 'calendar' | 'rank') {
  checkInTab.value = tab
  checkInDialogVisible.value = true
}

async function doCheckIn() {
  if (checkInTodayChecked.value) return
  checkInChecking.value = true
  try {
    const res = await forumApi.checkIn()
    ElMessage.success(`打卡成功，已连续 ${res.streak} 天`)
    checkInStreak.value = res.streak
    checkInTotal.value = res.total
    checkInTodayChecked.value = res.today_checked
  } catch (e) {
    console.error('打卡失败:', e)
  } finally {
    checkInChecking.value = false
  }
}

function onCheckInChecked(data: { streak: number; total: number; today_checked: boolean }) {
  checkInStreak.value = data.streak
  checkInTotal.value = data.total
  checkInTodayChecked.value = data.today_checked
}

onMounted(() => {
  if (route.query.tab === 'question') {
    categoryTab.value = 'question'
  }
  loadTopics()
  loadCheckInStatus()
})

watch(
  () => route.query.tab,
  (tab) => {
    if (tab === 'question' && categoryTab.value !== 'question') {
      categoryTab.value = 'question'
    } else if (tab === 'discussion' && categoryTab.value !== 'discussion') {
      categoryTab.value = 'discussion'
    }
  }
)
</script>
