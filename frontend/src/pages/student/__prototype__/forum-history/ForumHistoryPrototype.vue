<!-- PROTOTYPE — throwaway UI. Three variants of 论坛浏览记录, switchable via ?variant=a|b|c -->
<!-- Question: 浏览记录以何种形态融入论坛更适感？Variants: A Tab内联列表 / B 时间分组卡片网格 / C 紧凑清单+工具栏 -->
<template>
  <div class="forum-history-prototype">
    <div class="proto-banner">
      <el-tag size="small" type="warning" effect="plain">PROTOTYPE — throwaway</el-tag>
      <span class="proto-banner-text">
        论坛浏览记录 · 三变体对比（?variant=a|b|c，←/→ 切换）· 仅 ForumDetail 写入 · localStorage MRU 50 · 终态并入 ForumPage 第四 tab
      </span>
    </div>

    <div class="proto-header">
      <div>
        <h2 class="proto-title">论坛浏览记录</h2>
        <p class="proto-sub">
          共 {{ historyItems.length }} 条
          <span class="proto-sub-sep">·</span>
          最大 {{ maxHistory }} 条，MRU 去重，新浏览移至队首
          <span class="proto-sub-sep">·</span>
          key: <code class="proto-code">{{ storageKey }}</code>
        </p>
      </div>
      <div class="proto-actions">
        <el-button size="small" @click="handleSeed">写入示例 7 条</el-button>
        <el-button size="small" type="danger" plain :disabled="historyItems.length === 0" @click="handleClear">清空全部</el-button>
      </div>
    </div>

    <!-- 模拟浏览工具栏（演示写入 MRU 去重） -->
    <div class="simulate-bar">
      <span class="simulate-title">模拟浏览（点帖子→写入历史，重复则置顶）：</span>
      <div class="simulate-btns">
        <el-button
          v-for="t in mockTopics"
          :key="t.id"
          size="small"
          plain
          @click="simulateView(t)"
        >
          #{{ t.id }} {{ t.title.slice(0, 10) }}…
        </el-button>
      </div>
      <div class="simulate-extra">
        <el-button size="small" type="info" plain @click="simulateRandom">随机浏览一条</el-button>
        <el-button size="small" type="warning" plain :disabled="historyItems.length === 0" @click="simulateDeleteFirst">标记首条为已删除</el-button>
      </div>
    </div>

    <VariantA
      v-if="variant === 'a'"
      :items="historyItems"
      @select="handleSelect"
      @remove="handleRemove"
    />
    <VariantB
      v-else-if="variant === 'b'"
      :items="historyItems"
      @select="handleSelect"
      @remove="handleRemove"
    />
    <VariantC
      v-else
      :items="historyItems"
      @select="handleSelect"
      @remove="handleRemove"
      @clear="handleClear"
    />

    <div class="proto-state">
      <div class="proto-state-title">State</div>
      <div class="proto-state-grid">
        <div><span class="state-k">variant</span> {{ variant }}</div>
        <div><span class="state-k">count</span> {{ historyItems.length }} / {{ maxHistory }}</div>
        <div><span class="state-k">first</span> {{ historyItems[0] ? `#${historyItems[0].id} · ${historyItems[0].title} · ${historyItems[0].viewedAt}` : '—' }}</div>
        <div><span class="state-k">storageKey</span> {{ storageKey }}</div>
        <div class="state-raw">
          <span class="state-k">raw (前 3 条)</span>
          <pre class="state-pre">{{ rawPreview }}</pre>
        </div>
      </div>
    </div>

    <PrototypeSwitcher :variants="switcherVariants" :current="variant" />
  </div>
</template>

<script setup lang="ts">
import { computed, ref, onMounted } from 'vue'
import { useRoute } from 'vue-router'
import { ElMessage } from 'element-plus'
import PrototypeSwitcher from '@/components/prototype/PrototypeSwitcher.vue'
import VariantA from './VariantA.vue'
import VariantB from './VariantB.vue'
import VariantC from './VariantC.vue'
import type { MockTopic } from './types'
import {
  loadHistory,
  pushHistory,
  removeOne,
  clearAll,
  seedMockHistory,
  getStorageKey,
  getMaxHistory,
  markDeleted,
} from './utils'

const route = useRoute()
const variant = computed(() => {
  const v = String(route.query.variant || 'a').toLowerCase()
  return ['a', 'b', 'c'].includes(v) ? v : 'a'
})

const switcherVariants = [
  { key: 'a', label: 'Tab 内联列表' },
  { key: 'b', label: '时间分组卡片' },
  { key: 'c', label: '紧凑清单+工具栏' },
]

const historyItems = ref(loadHistory())
const storageKey = getStorageKey()
const maxHistory = getMaxHistory()

const mockTopics: MockTopic[] = [
  {
    id: 201,
    title: '实操考试时如何平稳叉取托盘？',
    content: '实操考试时叉取托盘总是晃动，有没有稳定的操作节奏和视线技巧？',
    author: { user_id: 11, username: '教练陈', avatar_url: '' },
    images_count: 1,
    view_count: 320,
    reply_count: 15,
  },
  {
    id: 202,
    title: '叉车仪表盘故障灯图解',
    content: '整理了一份常见故障灯含义图解，适合考前快速复习…',
    author: { user_id: 12, username: '老技师', avatar_url: '' },
    images_count: 2,
    view_count: 512,
    reply_count: 28,
  },
  {
    id: 203,
    title: '每日一练：安全操作选择题打卡',
    content: '每天 5 道安全操作选择题，坚持 30 天正确率提升明显…',
    author: { user_id: 13, username: '学习委员', avatar_url: '' },
    images_count: 0,
    view_count: 234,
    reply_count: 9,
  },
  {
    id: 204,
    title: '二手叉车购买避坑帖',
    content: '买二手叉车要注意车况、工时、品牌溢价，别只看价格…',
    author: { user_id: 14, username: '二手车商', avatar_url: '' },
    images_count: 3,
    view_count: 678,
    reply_count: 31,
  },
  {
    id: 205,
    title: 'N1 真题：液压与制动高频错题',
    content: '液压与制动章节的 12 道高频错题解析，附易错点总结…',
    author: { user_id: 15, username: '教研组', avatar_url: '' },
    images_count: 0,
    view_count: 445,
    reply_count: 19,
  },
]

function refresh() {
  historyItems.value = loadHistory()
}

function handleSeed() {
  seedMockHistory()
  refresh()
  ElMessage.success('已写入示例历史 7 条')
}

function handleClear() {
  clearAll()
  refresh()
  ElMessage.success('已清空')
}

function handleRemove(id: number) {
  removeOne(id)
  refresh()
  ElMessage.success(`已移除 #${id}`)
}

function simulateView(topic: MockTopic) {
  pushHistory({
    id: topic.id,
    title: topic.title,
    excerpt: topic.content,
    author: topic.author,
    images_count: topic.images_count,
    view_count: topic.view_count,
    reply_count: topic.reply_count,
  })
  refresh()
  ElMessage.info(`已浏览 #${topic.id}（MRU 置顶）`)
}

function simulateRandom() {
  const t = mockTopics[Math.floor(Math.random() * mockTopics.length)]
  simulateView(t)
}

function simulateDeleteFirst() {
  if (historyItems.value.length === 0) return
  const first = historyItems.value[0]
  markDeleted(first.id)
  refresh()
  ElMessage.warning(`已标记 #${first.id} 为已删除`)
}

function handleSelect(id: number) {
  const found = historyItems.value.find((h) => h.id === id)
  if (found?.deleted) {
    ElMessage.warning('原帖已删除')
    return
  }
  ElMessage.info(`查看帖子 #${id}（终态将跳 ForumDetail）`)
}

const rawPreview = computed(() => {
  const slice = historyItems.value.slice(0, 3)
  return JSON.stringify(slice, null, 2)
})

onMounted(() => {
  refresh()
  if (historyItems.value.length === 0) {
    handleSeed()
  }
})
</script>

<style scoped>
.forum-history-prototype {
  max-width: 1100px;
  margin: 0 auto;
  padding-bottom: 72px;
}
.proto-banner {
  display: flex;
  align-items: center;
  gap: 10px;
  margin-bottom: 16px;
  flex-wrap: wrap;
}
.proto-banner-text {
  font-size: 12px;
  color: var(--color-text-tertiary);
}
.proto-header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 16px;
  margin-bottom: 14px;
  flex-wrap: wrap;
}
.proto-title {
  font-size: var(--text-2xl);
  line-height: 1.2;
  margin: 0 0 6px;
}
.proto-sub {
  font-size: 13px;
  color: var(--color-text-tertiary);
  margin: 0;
}
.proto-sub-sep {
  margin: 0 6px;
  color: var(--color-text-muted);
}
.proto-code {
  font-family: var(--font-mono);
  font-size: 12px;
  background: var(--color-bg-page);
  padding: 1px 6px;
  border-radius: var(--radius-sm);
  border: 1px solid var(--color-border-light);
}
.proto-actions {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}
.simulate-bar {
  background: var(--color-bg-card);
  border: 1px solid var(--color-border-light);
  border-radius: var(--radius-lg);
  padding: 12px 14px;
  margin-bottom: 16px;
}
.simulate-title {
  font-size: 12px;
  font-weight: 600;
  color: var(--color-text-secondary);
  margin-bottom: 8px;
  display: block;
}
.simulate-btns {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  margin-bottom: 8px;
}
.simulate-extra {
  display: flex;
  gap: 8px;
  flex-wrap: wrap;
}
.proto-state {
  margin-top: 18px;
  padding: 12px 14px;
  background: var(--color-bg-card);
  border: 1px solid var(--color-border-light);
  border-radius: var(--radius-lg);
}
.proto-state-title {
  font-size: 12px;
  font-weight: 700;
  letter-spacing: 0.06em;
  color: var(--color-text-muted);
  margin-bottom: 8px;
  text-transform: uppercase;
}
.proto-state-grid {
  display: flex;
  flex-direction: column;
  gap: 4px;
  font-size: 12px;
  color: var(--color-text-secondary);
  font-family: var(--font-mono);
}
.state-k {
  color: var(--color-text-muted);
  margin-right: 6px;
}
.state-raw {
  margin-top: 8px;
}
.state-pre {
  margin: 6px 0 0;
  padding: 10px 12px;
  background: var(--color-bg-page);
  border: 1px solid var(--color-border-light);
  border-radius: var(--radius-md);
  font-size: 11px;
  line-height: 1.5;
  max-height: 220px;
  overflow: auto;
  white-space: pre-wrap;
  word-break: break-all;
}
</style>
