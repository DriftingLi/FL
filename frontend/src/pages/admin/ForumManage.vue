<!--
  列表档位：useAdminTable（分页列表）—— 帖子列表 / 举报队列
  列表档位：useAsyncPage（行内追加）—— 帖子展开面板的行内回复分页（按行实例，判据同源 utils/listState）
  依据 ADR-0056 §9；判定口径见 docs/agents/ui-conventions.md「管理端列表两档归属」。
-->
<template>
  <div class="forum-manage-page">
    <UiCard divider>
      <template #header>
        <span class="card-title">论坛管理</span>
      </template>
      <template #actions>
        <UiButton
          :icon="'rotate-cw'"
          circle
          aria-label="刷新"
          @click="activeMainTab === 'reports' ? loadReports() : loadList()"
        />
      </template>

      <el-tabs v-model="activeMainTab" @tab-change="handleMainTabChange">
        <el-tab-pane label="帖子管理" name="topics" />
        <el-tab-pane label="举报管理" name="reports" />
      </el-tabs>

      <!-- ===== 举报管理（ADR-0018）===== -->
      <template v-if="activeMainTab === 'reports'">
        <UiFilterBar>
        <template #filters>

          <UiRadioGroup v-model="reportStatus" @change="handleReportStatusChange">
            <el-radio-button :value="-1">全部</el-radio-button>
            <el-radio-button :value="0">待处理</el-radio-button>
            <el-radio-button :value="1">已处理</el-radio-button>
          </UiRadioGroup>
        </template>
      </UiFilterBar>

        <UiAsyncSection
          :error="reportError"
          :loading="reportLoading"
          :retrying="reportRetrying"
          :skeleton="false"
          error-title="举报队列加载失败"
          error-description="网络或服务端异常，可重试"
          @retry="retryReports"
        >
        <el-table v-loading="reportLoading" :data="reports" border>
          <el-table-column prop="id" label="ID" width="60" align="center" />
          <el-table-column label="举报人" width="110">
            <template #default="{ row }">{{ row.reporter || '-' }}</template>
          </el-table-column>
          <el-table-column label="对象" min-width="200">
            <template #default="{ row }">
              <UiTag size="small" :tone="row.reply_id ? 'info' : 'warning'">
                {{ row.reply_id ? '回复' : '帖子' }}
              </UiTag>
              <span class="report-target">{{ row.topic_title || `#${row.topic_id}` }}</span>
            </template>
          </el-table-column>
          <el-table-column prop="reason" label="举报理由" min-width="180" show-overflow-tooltip />
          <el-table-column label="状态" width="90" align="center">
            <template #default="{ row }">
              <UiTag size="small" :tone="row.status === 1 ? 'success' : 'danger'">
                {{ row.status === 1 ? '已处理' : '待处理' }}
              </UiTag>
            </template>
          </el-table-column>
          <el-table-column label="时间" width="160" align="center">
            <template #default="{ row }">{{ formatLocaleDateTime(row.created_at) }}</template>
          </el-table-column>
          <el-table-column label="操作" width="100" fixed="right" align="center">
            <template #default="{ row }">
              <UiButton variant="primary" v-if="row.status === 0" size="small" link @click="handleReport(row)">
                标记已处理
              </UiButton>
              <span v-else class="report-done">—</span>
            </template>
          </el-table-column>
        </el-table>
        </UiAsyncSection>

          <UiPagination v-if="reportTotal > reportPageSize"
      v-model:current-page="reportCurrentPage"
      :page-size="reportPageSize"
      :total="reportTotal"
      @current-change="loadReports"
    align="center" class="mt-4" />
        
      </template>

      <!-- ===== 帖子管理（原有内容）===== -->
      <template v-else>
      <UiFilterBar>
        <template #filters>

        <!-- 参数选择（同一列表换版块筛选）→ 胶囊，登记表「参数选择」行 -->
        <UiSegmentTabs v-model="activeTab" :options="topicTabOptions" @change="handleTabChange" />
        <UiInput v-model="keyword" placeholder="搜索标题 / 内容" clearable style="width: 260px" @clear="handleSearch" @keyup.enter="handleSearch">
          <template #prefix>
            <UiIcon name="search" />
          </template>
        </UiInput>
        </template>
      </UiFilterBar>

      <UiAsyncSection
        :error="loadError"
        :loading="loading"
        :retrying="retrying"
        :skeleton="false"
        error-title="帖子加载失败"
        error-description="网络或服务端异常，可重试"
        @retry="retryLoad"
      >
      <el-table
        v-loading="loading"
        :data="topics"
        stripe
        style="width: 100%"
        row-key="id"
      >
        <el-table-column label="标题" min-width="240">
          <template #default="{ row }">
            <div class="title-cell">
              <!-- 意图轴（学员自述，ADR-0040 只有两值） -->
              <UiTag v-if="row.category === 'question'" size="small" tone="success">问答</UiTag>
              <UiTag v-else-if="row.chapter_id" size="small" tone="warning">
                {{ row.chapter_title || '章节讨论' }}
              </UiTag>
              <UiTag v-else size="small" tone="info">综合</UiTag>
              <!-- 认定轴（管理端授予）：经验蕴含精选，故经验帖两个标签同时出现，
                   不合并、也绝不出现「经验但非精选」的误导态 -->
              <UiTag v-if="row.is_experience" size="small" tone="warning" effect="dark" class="font-semibold">备考经验</UiTag>
              <UiTag v-if="row.is_featured" size="small" effect="dark" class="font-semibold">★ 精选</UiTag>
              <button type="button" class="title-text cursor-pointer border-0 bg-transparent p-0 text-left underline-offset-2 hover:underline" @click="openTopic(row, $event)">
                {{ row.title }}
              </button>
            </div>
          </template>
        </el-table-column>
        <el-table-column prop="id" label="ID" width="70" align="center" />
        <el-table-column label="作者" min-width="120">
          <template #default="{ row }">{{ displayName(row.author) }}</template>
        </el-table-column>
        <el-table-column prop="reply_count" label="回复数" width="80" align="center" />
        <el-table-column prop="view_count" label="浏览" width="70" align="center" />
        <el-table-column label="创建时间" width="160" align="center">
          <template #default="{ row }">{{ formatLocaleDateTime(row.created_at) }}</template>
        </el-table-column>
        <el-table-column label="操作" width="90" fixed="right" align="center">
          <template #default="{ row }">
            <UiMoreMenu trigger="text" :items="topicItems(row)" @select="(k: string) => onTopicAction(k, row)" />
          </template>
        </el-table-column>
      </el-table>
      </UiAsyncSection>

        <UiPagination v-if="total > pageSize"
      v-model:current-page="currentPage"
      :page-size="pageSize"
      :total="total"
      @current-change="handlePageChange"
    align="center" class="mt-4" />
      
      </template>
      </UiCard>
    <el-drawer v-model="topicDrawerOpen" :title="selectedTopic?.title" size="min(640px, 100vw)" @closed="restoreTopicFocus">
      <template v-if="selectedTopic">
        <div class="topic-content">
          <!-- 治理面看**实际展示效果**（ADR-0044）：渲染版才看得出学员最终看到的是什么。
               raw HTML 在 escape 策略下以文本形式可见，所以渲染版不会掩盖藏起来的标记。 -->
          <ForumContent
            :content="selectedTopic.content"
            :format="toForumContentFormat(selectedTopic.content_format)"
            link-policy="plain"
            class="topic-content-text"
          />
          <ForumImageGallery :images="selectedTopic.images" />
        </div>
        <UiAsyncSection
          :error="selectedReplyPages?.error ?? false"
          :loading="selectedReplyPages?.loading ?? false"
          :skeleton="false"
          error-title="回复加载失败"
          error-description="网络或服务端异常，可重试"
          @retry="loadReplies(selectedTopic.id)"
        >
          <div v-loading="selectedReplyPages?.loading ?? false" class="pt-2">
            <template v-if="selectedReplyPages?.replies">
              <div v-if="selectedReplyPages.replies.length > 0" class="reply-list">
                <div v-for="reply in selectedReplyPages.replies" :key="reply.id" class="reply-item">
                  <div class="reply-meta">
                    <el-avatar :size="24" :src="reply.author.avatar_url || undefined">
                      {{ displayName(reply.author).charAt(0).toUpperCase() }}
                    </el-avatar>
                    <span class="reply-author">{{ displayName(reply.author) }}</span>
                    <!-- 与学员端一致的「› 被回复人」行内形态（ADR-0042），不再用独立引用块 -->
                    <span v-if="reply.parent_id && reply.parent_name" class="reply-quote">
                      › {{ reply.parent_name }}
                    </span>
                    <span class="reply-time">{{ formatLocaleDateTime(reply.created_at) }}</span>
                    <UiButton variant="danger" class="reply-delete" size="small" @click="deleteReply(reply)">
                      删除回复
                    </UiButton>
                  </div>
                  <!-- 回复同样按声明格式渲染：管理员判断违规依据的是学员实际看到的样子 -->
                  <ForumContent
                    :content="reply.content"
                    :format="toForumContentFormat(reply.content_format)"
                    link-policy="plain"
                    class="reply-content"
                  />
                  <ForumImageGallery :images="reply.images" />
                </div>
              </div>
              <!-- 回复分页（ADR-0042）：治理面必须能翻到底，不能只看首页 -->
              <div v-if="selectedReplyPages.page < selectedReplyPages.pages" class="reply-more">
                <span v-if="selectedReplyPages.moreError" class="reply-error">加载失败，可重试</span>
                <UiButton
                  size="small"
                  :loading="selectedReplyPages.loadingMore"
                  @click="loadMoreReplies(selectedTopic.id)"
                >
                  加载更多回复（剩余 {{ Math.max(selectedReplyPages.total - selectedReplyPages.replies.length, 0) }} 条）
                </UiButton>
              </div>
              <UiEmptyState v-else-if="isEmptyReplies(selectedTopic.id)" description="暂无回复" size="sm" />
            </template>
            <div v-else class="reply-loading">加载中…</div>
          </div>
        </UiAsyncSection>
      </template>
    </el-drawer>
  </div>
</template>

<script setup lang="ts">
import UiIcon from '@/components/ui/UiIcon.vue'
// keep-alive 的 include 按**组件名**匹配：名必须与路由名（页面描述符的 name）一致，
// 否则该页不会被缓存（#1620 的锁测试会拦下这种漂移）。
defineOptions({ name: 'ForumManage' })

import { computed, ref, onMounted } from 'vue'
import { ElMessage } from 'element-plus'
import {
  adminForumApi,
  forumTabQuery,
  toForumContentFormat,
  type ForumTab,
  type AdminForumTopic,
  type AdminForumReply,
  type AdminForumReportItem
} from '@/api/forum'
import ForumImageGallery from '@/components/student/ForumImageGallery.vue'
import ForumContent from '@/components/student/ForumContent.vue'
import { formatLocaleDateTime } from '@/utils/format'
import { useAdminTable } from '@/composables/useAdminTable'
import { useForumReplyPages } from './useForumReplyPages'
import UiButton from '@/components/ui/UiButton.vue'
import UiAsyncSection from '@/components/ui/UiAsyncSection.vue'
import UiEmptyState from '@/components/ui/UiEmptyState.vue'
import UiPagination from '@/components/ui/UiPagination.vue'
import UiFilterBar from '@/components/ui/UiFilterBar.vue'
import { useConfirm } from '@/composables/useConfirm'
import UiTag from '@/components/ui/UiTag.vue'
import UiRadioGroup from '@/components/ui/UiRadioGroup.vue'
import UiMoreMenu, { type UiMoreMenuItem } from '@/components/ui/UiMoreMenu.vue'
import UiCard from '@/components/ui/UiCard.vue'
import UiSegmentTabs from '@/components/ui/UiSegmentTabs.vue'
import UiInput from '@/components/ui/UiInput.vue'

// 帖子列表：admin 列表状态机 useAdminTable（#792，ADR-0039）——三态 + 分页 + 列表一并托管。
// 解构改名保持模板零改动；keyword 为页面自管筛选轴（由 fetch adapter 读取）。
const {
  loading,
  loadError,
  retrying,
  list: topics,
  total,
  currentPage,
  pageSize,
  load: loadList,
  retry: retryLoad
} = useAdminTable<AdminForumTopic>({
  pageSize: 10,
  fetch: (paging) =>
    adminForumApi.listTopics({
      ...forumTabQuery(activeTab.value),
      page: paging.page,
      page_size: paging.pageSize,
      keyword: keyword.value || undefined
    })
})

/** 分页控件回调：useAdminTable 的 load 读取 currentPage，翻页后重装 */
function handlePageChange(): void {
  void loadList()
}
// 管理端筛选轴：all=全部帖子、discussion=综合讨论区、question=问答区、experience=备考经验认定（#742 走查补齐）。
// 四个值都交给 forumTabQuery 翻译成查询参数——"综合讨论区必须带 category=discussion"
// 这条规则只在 api 层写一遍，学员端与管理端共用同一份映射。
// ⚠️ experience 翻译出的是 scope=all + is_experience=true（管理端认定，ADR-0040）：
// category='experience' 的存量行已由迁移降级为 discussion，发它必然空。
const activeTab = ref<ForumTab>('all')
const keyword = ref('')
const selectedTopic = ref<AdminForumTopic | null>(null)
const topicDrawerOpen = ref(false)
let topicTrigger: HTMLElement | null = null
const {
  replyPages, loadReplies, loadMoreReplies, refreshReplies, invalidateReplies, isEmptyReplies
} = useForumReplyPages()
const selectedReplyPages = computed(() => selectedTopic.value && replyPages[selectedTopic.value.id])

// ===== 举报管理（ADR-0018）=====
// 档位一：分页列表 → useAdminTable（同页第二个实例）；reportStatus 是页面自管筛选轴。
const activeMainTab = ref<'topics' | 'reports'>('topics')
const reportStatus = ref(-1)
const {
  loading: reportLoading,
  loadError: reportError,
  retrying: reportRetrying,
  list: reports,
  total: reportTotal,
  currentPage: reportCurrentPage,
  pageSize: reportPageSize,
  load: loadReports,
  retry: retryReports
} = useAdminTable<AdminForumReportItem>({
  pageSize: 20,
  fetch: (paging) =>
    adminForumApi.listReports({
      status: reportStatus.value >= 0 ? reportStatus.value : undefined,
      page: paging.page,
      page_size: paging.pageSize
    })
})

function handleMainTabChange(tab: string | number) {
  if (tab === 'reports' && reports.value.length === 0) {
    loadReports()
  }
}

function handleReportStatusChange() {
  reportCurrentPage.value = 1
  loadReports()
}

async function handleReport(row: AdminForumReportItem) {
  try {
    await useConfirm().confirm('确认将该举报标记为已处理？', '处理举报', { type: 'warning' })
  } catch {
    return
  }
  try {
    await adminForumApi.handleReport(row.id, 1)
    ElMessage.success('已标记处理')
    loadReports()
  } catch (e) {
    console.error('处理举报失败:', e)
    /* 错误已由拦截器提示 */
  }
}

function displayName(author: AdminForumTopic['author']) {
  return author.username
}

const topicTabOptions = [
  { label: '全部帖子', value: 'all' },
  { label: '综合讨论区', value: 'discussion' },
  { label: '问答区', value: 'question' },
  { label: '备考经验', value: 'experience' }
]

function handleTabChange() {
  currentPage.value = 1
  loadList()
}

function handleSearch() {
  currentPage.value = 1
  loadList()
}

function openTopic(topic: AdminForumTopic, event: MouseEvent) {
  topicTrigger = event.currentTarget instanceof HTMLElement ? event.currentTarget : null
  selectedTopic.value = topic
  topicDrawerOpen.value = true
  void loadReplies(topic.id)
}

function restoreTopicFocus() {
  if (topicDrawerOpen.value) return
  selectedTopic.value = null
  if (topicTrigger?.isConnected) topicTrigger.focus({ preventScroll: true })
  topicTrigger = null
}

/**
 * 帖子表的行内「操作」菜单。
 * 「经验」与「精选」互斥：经验帖不得直接撤精（后端 400），所以那项置灰 + 给一条逃生说明，
 * 而不是把它藏掉 —— 藏掉会让人以为没这个操作。
 */
function topicItems(row: AdminForumTopic): UiMoreMenuItem[] {
  const items: UiMoreMenuItem[] = [
    row.is_experience
      ? { key: 'revokeExperience', label: '取消经验认定' }
      : { key: 'designateExperience', label: '认定经验' }
  ]
  if (row.is_featured && row.is_experience) {
    items.push({
      key: 'unfeatureTopic',
      label: '取消精选',
      disabled: true,
      hint: '备考经验帖蕴含精选位，请先取消经验认定'
    })
  } else if (row.is_featured) {
    items.push({ key: 'unfeatureTopic', label: '取消精选' })
  } else {
    items.push({ key: 'featureTopic', label: '加精' })
  }
  items.push({ key: 'delete', label: '删除', tone: 'danger', divided: true })
  return items
}

interface ForumActionDescriptor<T> {
  confirmation: {
    danger?: boolean
    message: (target: T) => string
    title: string
    type: 'info' | 'warning'
  }
  request: (target: T) => Promise<unknown>
  successMessage: string
  errorLabel: string
  onSuccess?: (target: T) => void
}

async function executeForumAction<T>(action: ForumActionDescriptor<T>, target: T): Promise<void> {
  try {
    const confirmation = useConfirm()
    const { message, title, type, danger } = action.confirmation
    if (danger) {
      await confirmation.confirmDanger(message(target), title, { type })
    } else {
      await confirmation.confirm(message(target), title, { type })
    }
  } catch {
    return
  }
  try {
    await action.request(target)
    ElMessage.success(action.successMessage)
    action.onSuccess?.(target)
    loadList()
  } catch (e) {
    console.error(`${action.errorLabel}失败:`, e)
    /* 错误已由拦截器提示 */
  }
}

const topicActionDescriptors = new Map<string, ForumActionDescriptor<AdminForumTopic>>([
  ['designateExperience', {
    confirmation: {
      message: row => `认定后「${row.title}」将进入备考经验区并带上精选标识，帖主获 +30 分（每帖仅一次；已加精的帖子不重复发放）。`,
      title: '认定备考经验',
      type: 'info'
    },
    request: row => adminForumApi.designateExperience(row.id),
    successMessage: '已认定为备考经验',
    errorLabel: '认定备考经验'
  }],
  ['revokeExperience', {
    confirmation: {
      message: row => `确定取消「${row.title}」的备考经验认定？精选位保留，已发放积分不收回。`,
      title: '取消经验认定',
      type: 'info'
    },
    request: row => adminForumApi.revokeExperience(row.id),
    successMessage: '已取消经验认定',
    errorLabel: '取消经验认定'
  }],
  ['featureTopic', {
    confirmation: {
      message: row => `加精后「${row.title}」将带上精选标识，帖主获 +30 分（每帖仅一次）。`,
      title: '加精帖子',
      type: 'info'
    },
    request: row => adminForumApi.featureTopic(row.id),
    successMessage: '已加精',
    errorLabel: '加精'
  }],
  ['unfeatureTopic', {
    confirmation: {
      message: row => `确定取消「${row.title}」的精选标识？`,
      title: '取消精选',
      type: 'info'
    },
    request: row => adminForumApi.unfeatureTopic(row.id),
    successMessage: '已取消精选',
    errorLabel: '取消精选'
  }],
  ['delete', {
    confirmation: {
      danger: true,
      message: row => `确定删除帖子「${row.title}」？删除后不可恢复。`,
      title: '删除帖子',
      type: 'warning'
    },
    request: row => adminForumApi.deleteTopic(row.id),
    successMessage: '已删除',
    errorLabel: '删除',
    onSuccess: row => {
      invalidateReplies(row.id)
    }
  }]
])

function onTopicAction(key: string, row: AdminForumTopic) {
  const action = topicActionDescriptors.get(key)
  if (action) void executeForumAction(action, row)
}

const deleteReplyAction: ForumActionDescriptor<AdminForumReply> = {
  confirmation: {
    danger: true,
    message: () => '确定删除这条回复？删除后不可恢复。',
    title: '删除回复',
    type: 'warning'
  },
  request: reply => adminForumApi.deleteReply(reply.id),
  successMessage: '已删除',
  errorLabel: '删除',
  onSuccess: reply => {
    void refreshReplies(reply.topic_id)
  }
}

function deleteReply(reply: AdminForumReply) {
  return executeForumAction(deleteReplyAction, reply)
}

onMounted(loadList)
</script>

<style scoped>
.card-title {
  font-size: var(--text-base);
  font-weight: 600;
}

.report-target {
  margin-left: 8px;
  font-size: var(--text-sm);
}

.report-done {
  color: var(--color-text-disabled);
}


.title-cell {
  display: flex;
  align-items: center;
  gap: 8px;
}

.title-text {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.topic-content {
  margin-bottom: 14px;
  padding-bottom: 14px;
  border-bottom: 1px solid var(--color-border-light);
}

/* 只管本页的密度（字号/行高/色），**不再管换行与断词**——
   那两条现在由 ForumContent 的原子类按格式分支决定（纯文本才保留换行）。
   同一元素同一属性既有 scoped 类又有原子类会构成双写（ui-conventions R3）。 */
.topic-content-text {
  font-size: var(--text-sm);
  color: var(--color-text-primary);
  line-height: 1.7;
}

.reply-list {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

/* 回复分页的「加载更多」入口（ADR-0042）：居中，与列表拉开一点距离 */
.reply-more {
  display: flex;
  justify-content: center;
  align-items: center;
  gap: 8px;
  padding-top: 4px;
}

/* 行内追加形态的失败提示（#1102）：与按钮同一行，按钮本身即重试入口 */
.reply-error {
  font-size: var(--text-xs);
  color: var(--color-danger);
}

.reply-item {
  border-bottom: 1px solid var(--color-bg-page);
  padding-bottom: 10px;
}

.reply-item:last-child {
  border-bottom: none;
  padding-bottom: 0;
}

.reply-meta {
  display: flex;
  align-items: center;
  gap: 8px;
}

.reply-author {
  font-weight: 600;
  color: var(--color-text-primary);
}

.reply-quote {
  font-size: var(--text-xs);
  color: var(--color-text-tertiary);
  background: var(--color-bg-page);
  border-radius: var(--radius-md);
  padding: 1px 6px;
}

.reply-time {
  font-size: var(--text-xs);
  color: var(--color-text-tertiary);
}

.reply-delete {
  margin-left: auto;
}

.reply-content {
  margin-top: 4px;
  color: var(--color-text-primary);
  font-size: var(--text-sm);
  white-space: pre-wrap;
  word-break: break-word;
}

.reply-loading {
  color: var(--color-text-tertiary);
  font-size: var(--text-sm);
}

</style>
