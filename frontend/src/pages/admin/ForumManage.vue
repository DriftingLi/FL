<template>
  <div class="forum-manage-page">
    <el-card>
      <template #header>
        <div class="card-header">
          <span class="card-title">论坛管理</span>
          <UiButton :icon="Refresh" circle @click="activeMainTab === 'reports' ? loadReports() : loadList()"/>
        </div>
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

        <el-tabs v-model="activeTab" @tab-change="handleTabChange">
          <el-tab-pane label="全部帖子" name="all" />
          <el-tab-pane label="综合讨论区" name="discussion" />
          <el-tab-pane label="问答区" name="question" />
          <el-tab-pane label="备考经验" name="experience" />
        </el-tabs>
        <el-input
          v-model="keyword"
          placeholder="搜索标题 / 内容"
          clearable
          style="width: 260px"
          @clear="handleSearch"
          @keyup.enter="handleSearch"
        >
          <template #prefix>
            <el-icon><Search /></el-icon>
          </template>
        </el-input>
        </template>
      </UiFilterBar>

      <UiErrorState
        v-if="loadError"
        title="帖子加载失败"
        description="网络或服务端异常，可重试"
        :retrying="retrying"
        @retry="retryLoad"
      />
      <el-table
        v-else
        v-loading="loading"
        :data="topics"
        stripe
        style="width: 100%"
        row-key="id"
        :expand-row-keys="expandedRows"
        @expand-change="handleExpand"
      >
        <el-table-column type="expand" width="40">
          <template #default="{ row }">
            <div v-loading="detailLoadingId === row.id" class="expand-replies">
              <template v-if="replyMap[row.id]">
                <div class="topic-content">
                  <!-- 治理面看**实际展示效果**（ADR-0044）：渲染版才看得出学员最终看到的是什么。
                       raw HTML 在 escape 策略下以文本形式可见，所以渲染版不会掩盖藏起来的标记。 -->
                  <ForumContent
                    :content="row.content"
                    :format="row.content_format"
                    class="topic-content-text"
                  />
                  <ForumImageGallery :images="row.images" />
                </div>
                <div v-if="replyMap[row.id].length > 0" class="reply-list">
                  <div v-for="reply in replyMap[row.id]" :key="reply.id" class="reply-item">
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
                    <div class="reply-content">{{ reply.content }}</div>
                    <ForumImageGallery :images="reply.images" />
                  </div>
                </div>
                <!-- 回复分页（ADR-0042）：治理面必须能翻到底，不能只看首页 -->
                <div v-if="replyMap[row.id] && hasMoreReplies(row.id)" class="reply-more">
                  <UiButton
                    size="small"
                    :loading="replyLoadingMoreIds.includes(row.id)"
                    @click="loadMoreReplies(row.id)"
                  >
                    加载更多回复（剩余 {{ remainingRepliesOf(row.id) }} 条）
                  </UiButton>
                </div>
                <UiEmptyState v-else-if="!replyMap[row.id] || replyMap[row.id].length === 0" description="暂无回复" size="sm" />
              </template>
              <div v-else class="reply-loading">加载中…</div>
            </div>
          </template>
        </el-table-column>
        <el-table-column prop="id" label="ID" width="70" align="center" />
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
              <span class="title-text">{{ row.title }}</span>
            </div>
          </template>
        </el-table-column>
        <el-table-column label="作者" min-width="120">
          <template #default="{ row }">{{ displayName(row.author) }}</template>
        </el-table-column>
        <el-table-column prop="reply_count" label="回复数" width="80" align="center" />
        <el-table-column prop="view_count" label="浏览" width="70" align="center" />
        <el-table-column label="创建时间" width="160" align="center">
          <template #default="{ row }">{{ formatLocaleDateTime(row.created_at) }}</template>
        </el-table-column>
        <el-table-column label="操作" width="230" fixed="right" align="center">
          <template #default="{ row }">
            <div class="flex items-center justify-center gap-1.5">
              <UiButton
                v-if="!row.is_experience"
                variant="secondary"
                size="small"
                @click="designateExperience(row)"
              >认定经验</UiButton>
              <UiButton
                v-else
                variant="secondary"
                size="small"
                @click="revokeExperience(row)"
              >取消经验认定</UiButton>
              <!-- 经验蕴含精选：经验帖不得直接撤精（后端 400），禁用并给逃生口 -->
              <UiTooltip v-if="row.is_featured && row.is_experience" content="备考经验帖蕴含精选位，请先取消经验认定">
                <span>
                  <UiButton variant="secondary" size="small" disabled>取消精选</UiButton>
                </span>
              </UiTooltip>
              <UiButton
                v-else-if="!row.is_featured"
                variant="secondary"
                size="small"
                @click="featureTopic(row)"
              >加精</UiButton>
              <UiButton
                v-else
                variant="secondary"
                size="small"
                @click="unfeatureTopic(row)"
              >取消精选</UiButton>
              <UiButton variant="danger" size="small" @click="deleteTopic(row)">删除</UiButton>
            </div>
          </template>
        </el-table-column>
      </el-table>

        <UiPagination v-if="total > pageSize"
      v-model:current-page="currentPage"
      :page-size="pageSize"
      :total="total"
      @current-change="handlePageChange"
    align="center" class="mt-4" />
      
      </template>
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { ElMessage } from 'element-plus'
import { Refresh, Search } from '@element-plus/icons-vue'
import {
  adminForumApi,
  forumTabQuery,
  type ForumTab,
  type AdminForumTopic,
  type AdminForumReply,
  type AdminForumReportItem
} from '@/api/forum'
import ForumImageGallery from '@/components/student/ForumImageGallery.vue'
import ForumContent from '@/components/student/ForumContent.vue'
import { formatLocaleDateTime } from '@/utils/format'
import { useAdminTable } from '@/composables/useAdminTable'
import UiButton from '@/components/ui/UiButton.vue'
import UiEmptyState from '@/components/ui/UiEmptyState.vue'
import UiErrorState from '@/components/ui/UiErrorState.vue'
import UiPagination from '@/components/ui/UiPagination.vue'
import UiFilterBar from '@/components/ui/UiFilterBar.vue'
import { useConfirm } from '@/composables/useConfirm'
import UiTag from '@/components/ui/UiTag.vue'
import UiRadioGroup from '@/components/ui/UiRadioGroup.vue'
import UiTooltip from '@/components/ui/UiTooltip.vue'

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
  fetch: async (paging) => {
    const res = await adminForumApi.listTopics({
      ...forumTabQuery(activeTab.value),
      page: paging.page,
      page_size: paging.pageSize,
      keyword: keyword.value || undefined
    })
    return { list: res.topics || [], total: res.total || 0 }
  }
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
const expandedRows = ref<number[]>([])
const replyMap = ref<Record<number, AdminForumReply[]>>({})
const detailLoadingId = ref<number | null>(null)
// 回复分页（ADR-0042）：详情接口分页返回，治理面必须能翻到底——
// 只渲染首页会让管理员看不见后面的违规回复。
const ADMIN_REPLY_PAGE_SIZE = 20
const replyMeta = ref<Record<number, { page: number; pages: number; total: number }>>({})
// 按行记加载态：单个全局 id 会让「另一行同时点加载更多」被静默忽略
const replyLoadingMoreIds = ref<number[]>([])

// ===== 举报管理（ADR-0018）=====
const activeMainTab = ref<'topics' | 'reports'>('topics')
const reportLoading = ref(false)
const reports = ref<AdminForumReportItem[]>([])
const reportTotal = ref(0)
const reportCurrentPage = ref(1)
const reportPageSize = ref(20)
const reportStatus = ref(-1)

function handleMainTabChange(tab: string | number) {
  if (tab === 'reports' && reports.value.length === 0) {
    loadReports()
  }
}

function handleReportStatusChange() {
  reportCurrentPage.value = 1
  loadReports()
}

async function loadReports() {
  reportLoading.value = true
  try {
    const res = await adminForumApi.listReports({
      status: reportStatus.value >= 0 ? reportStatus.value : undefined,
      page: reportCurrentPage.value,
      page_size: reportPageSize.value
    })
    reports.value = res.reports || []
    reportTotal.value = res.total || 0
  } catch (e) {
    console.error('加载举报列表失败:', e)
    /* 错误已由拦截器提示 */
  } finally {
    reportLoading.value = false
  }
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

function handleTabChange() {
  currentPage.value = 1
  loadList()
}

function handleSearch() {
  currentPage.value = 1
  loadList()
}

async function handleExpand(row: AdminForumTopic, expandedRowsNow: AdminForumTopic[]) {
  const expanded = expandedRowsNow.map(r => r.id)
  expandedRows.value = expanded
  if (expanded.includes(row.id) && !replyMap.value[row.id]) {
    await loadReplies(row.id)
  }
}

async function loadReplies(topicId: number) {
  detailLoadingId.value = topicId
  try {
    const res = await adminForumApi.getTopic(topicId, 1, ADMIN_REPLY_PAGE_SIZE)
    replyMap.value = { ...replyMap.value, [topicId]: res.replies || [] }
    replyMeta.value = {
      ...replyMeta.value,
      [topicId]: { page: res.page ?? 1, pages: res.pages ?? 1, total: res.total ?? 0 }
    }
  } catch (e) {
    console.error('加载回复失败:', e)
    /* 错误已由拦截器提示 */
  } finally {
    detailLoadingId.value = null
  }
}

/** 该帖是否还有未加载的回复页 */
function hasMoreReplies(topicId: number) {
  const meta = replyMeta.value[topicId]
  return !!meta && meta.page < meta.pages
}

/** 尚未加载的回复条数 */
function remainingRepliesOf(topicId: number) {
  const meta = replyMeta.value[topicId]
  if (!meta) return 0
  return Math.max(meta.total - (replyMap.value[topicId]?.length ?? 0), 0)
}

/** 加载更多回复：**追加**下一页（不替换） */
async function loadMoreReplies(topicId: number) {
  if (replyLoadingMoreIds.value.includes(topicId) || !hasMoreReplies(topicId)) return
  replyLoadingMoreIds.value = [...replyLoadingMoreIds.value, topicId]
  try {
    const next = (replyMeta.value[topicId]?.page ?? 1) + 1
    const res = await adminForumApi.getTopic(topicId, next, ADMIN_REPLY_PAGE_SIZE)
    replyMap.value = {
      ...replyMap.value,
      [topicId]: [...(replyMap.value[topicId] ?? []), ...(res.replies || [])]
    }
    replyMeta.value = {
      ...replyMeta.value,
      [topicId]: {
        page: res.page ?? next,
        pages: res.pages ?? replyMeta.value[topicId]?.pages ?? 1,
        total: res.total ?? replyMeta.value[topicId]?.total ?? 0
      }
    }
  } catch (e) {
    console.error('加载更多回复失败:', e)
    /* 错误已由拦截器提示 */
  } finally {
    replyLoadingMoreIds.value = replyLoadingMoreIds.value.filter((id) => id !== topicId)
  }
}

async function deleteTopic(row: AdminForumTopic) {
  try {
    await useConfirm().confirmDanger(`确定删除帖子「${row.title}」？删除后不可恢复。`, '删除帖子', { type: 'warning' })
  } catch {
    return
  }
  try {
    await adminForumApi.deleteTopic(row.id)
    ElMessage.success('已删除')
    delete replyMap.value[row.id]
    loadList()
  } catch (e) {
    console.error('删除失败:', e)
    /* 错误已由拦截器提示 */
  }
}

// ===== 精选位（#742）：全类别可精/可撤；首次加精后端同事务发帖主 +30（幂等） =====

async function featureTopic(row: AdminForumTopic) {
  try {
    await useConfirm().confirm(`加精后「${row.title}」将带上精选标识，帖主获 +30 分（每帖仅一次）。`, '加精帖子', { type: 'info' })
  } catch {
    return
  }
  try {
    await adminForumApi.featureTopic(row.id)
    ElMessage.success('已加精')
    loadList()
  } catch (e) {
    console.error('加精失败:', e)
    /* 错误已由拦截器提示 */
  }
}

async function unfeatureTopic(row: AdminForumTopic) {
  try {
    await useConfirm().confirm(`确定取消「${row.title}」的精选标识？`, '取消精选', { type: 'info' })
  } catch {
    return
  }
  try {
    await adminForumApi.unfeatureTopic(row.id)
    ElMessage.success('已取消精选')
    loadList()
  } catch (e) {
    console.error('取消精选失败:', e)
    /* 错误已由拦截器提示 */
  }
}

// ===== 备考经验认定（ADR-0040）：与精选位同轴的管理端认定，经验蕴含精选 =====

async function designateExperience(row: AdminForumTopic) {
  try {
    await useConfirm().confirm(
      `认定后「${row.title}」将进入备考经验区并带上精选标识，帖主获 +30 分（每帖仅一次；已加精的帖子不重复发放）。`,
      '认定备考经验',
      { type: 'info' }
    )
  } catch {
    return
  }
  try {
    await adminForumApi.designateExperience(row.id)
    ElMessage.success('已认定为备考经验')
    loadList()
  } catch (e) {
    console.error('认定备考经验失败:', e)
    /* 错误已由拦截器提示 */
  }
}

async function revokeExperience(row: AdminForumTopic) {
  try {
    await useConfirm().confirm(
      `确定取消「${row.title}」的备考经验认定？精选位保留，已发放积分不收回。`,
      '取消经验认定',
      { type: 'info' }
    )
  } catch {
    return
  }
  try {
    await adminForumApi.revokeExperience(row.id)
    ElMessage.success('已取消经验认定')
    loadList()
  } catch (e) {
    console.error('取消经验认定失败:', e)
    /* 错误已由拦截器提示 */
  }
}

async function deleteReply(reply: AdminForumReply) {
  try {
    await useConfirm().confirmDanger('确定删除这条回复？删除后不可恢复。', '删除回复', { type: 'warning' })
  } catch {
    return
  }
  try {
    await adminForumApi.deleteReply(reply.id)
    ElMessage.success('已删除')
    if (replyMap.value[reply.topic_id]) {
      replyMap.value = {
        ...replyMap.value,
        [reply.topic_id]: replyMap.value[reply.topic_id].filter(r => r.id !== reply.id)
      }
      // total 要一起减，否则「剩余 N 条」比实际多（删父回复会级联删子树，实际减得更多，
      // 但那是下一次加载才拿得到的真值——这里只保证不会**虚高**）。
      const meta = replyMeta.value[reply.topic_id]
      if (meta) {
        replyMeta.value = {
          ...replyMeta.value,
          [reply.topic_id]: { ...meta, total: Math.max(meta.total - 1, 0) }
        }
      }
    }
    loadList()
  } catch (e) {
    console.error('删除失败:', e)
    /* 错误已由拦截器提示 */
  }
}

onMounted(loadList)
</script>

<style scoped>
.card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.card-title {
  font-size: 16px;
  font-weight: 600;
}

.report-target {
  margin-left: 8px;
  font-size: 13px;
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

.expand-replies {
  padding: 8px 20px 8px 60px;
}

.topic-content {
  margin-bottom: 14px;
  padding-bottom: 14px;
  border-bottom: 1px solid var(--color-border-light);
}

.topic-content-text {
  font-size: 14px;
  color: var(--color-text-primary);
  line-height: 1.7;
  white-space: pre-wrap;
  word-break: break-word;
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
  padding-top: 4px;
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
  font-size: 12px;
  color: var(--color-text-tertiary);
  background: var(--color-bg-page);
  border-radius: 6px;
  padding: 1px 6px;
}

.reply-time {
  font-size: 12px;
  color: var(--color-text-tertiary);
}

.reply-delete {
  margin-left: auto;
}

.reply-content {
  margin-top: 4px;
  color: var(--color-text-primary);
  font-size: 13px;
  white-space: pre-wrap;
  word-break: break-word;
}

.reply-loading {
  color: var(--color-text-tertiary);
  font-size: 13px;
}

</style>
