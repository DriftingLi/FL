<!-- PROTOTYPE — throwaway UI. Three variants of 真题练习二级列表占位, switchable via ?variant=a|b|c on /training/__prototype/real-exam -->
<!-- Question: 套卷列表占位何种布局更适感？Variants: A 紧凑卡片网格 / B 列表+筛选头 / C 年份分组时间线 -->
<template>
  <div class="real-exam-prototype">
    <div class="proto-banner">
      <el-tag size="small" type="warning" effect="plain">PROTOTYPE — throwaway</el-tag>
      <span class="proto-banner-text">真题练习 · 占位列表三变体对比（?variant=a|b|c，←/→ 切换） · 点击套卷仅提示“功能建设中”</span>
    </div>

    <div class="proto-header">
      <div>
        <h2 class="proto-title">真题练习</h2>
        <p class="proto-sub">
          当前证件：<strong>{{ credentialStore.current?.name || currentCredentialName }}</strong>
          <span class="proto-sub-sep">·</span>
          已展示 {{ filteredPapers.length }} / {{ papers.length }} 套（按当前证件过滤）
        </p>
      </div>
      <el-select v-model="filterCredentialId" size="small" style="width: 200px" placeholder="切换证件（原型演示）">
        <el-option v-for="c in credentialOptions" :key="c.id" :label="c.name" :value="c.id" />
      </el-select>
    </div>

    <VariantA
      v-if="variant === 'a'"
      :papers="filteredPapers"
      :current-credential-name="currentCredentialName"
      :selected-id="selectedId"
      @select="handleSelect"
    />
    <VariantB
      v-else-if="variant === 'b'"
      :papers="filteredPapers"
      :current-credential-name="currentCredentialName"
      :selected-id="selectedId"
      :year-filter="filterYear"
      :difficulty-filter="filterDifficulty"
      :keyword="keyword"
      :year-options="yearOptions"
      @update:year-filter="filterYear = $event"
      @update:difficulty-filter="filterDifficulty = $event"
      @update:keyword="keyword = $event"
      @select="handleSelect"
      @clear="clearFilters"
    />
    <VariantC
      v-else
      :papers="filteredPapers"
      :current-credential-name="currentCredentialName"
      :selected-id="selectedId"
      @select="handleSelect"
    />

    <div class="proto-state">
      <div class="proto-state-title">State</div>
      <div class="proto-state-grid">
        <div><span class="state-k">variant</span> {{ variant }}</div>
        <div><span class="state-k">credential</span> {{ currentCredentialName }} (id={{ filterCredentialId }})</div>
        <div><span class="state-k">filters</span> year={{ filterYear ?? '—' }} · difficulty={{ filterDifficulty ?? '—' }} · keyword={{ keyword || '—' }}</div>
        <div><span class="state-k">selected</span> {{ selectedPaper ? `${selectedPaper.year} · ${selectedPaper.title} (#${selectedPaper.id})` : '—' }}</div>
        <div><span class="state-k">filtered</span> {{ filteredPapers.length }} / {{ papers.length }}</div>
      </div>
    </div>

    <PrototypeSwitcher :variants="switcherVariants" :current="variant" />
  </div>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import { ElMessage } from 'element-plus'
import { useCredentialStore } from '@/stores/credential'
import PrototypeSwitcher from '@/components/prototype/PrototypeSwitcher.vue'
import VariantA from './VariantA.vue'
import VariantB from './VariantB.vue'
import VariantC from './VariantC.vue'
import type { Paper, Difficulty } from './types'

const credentialStore = useCredentialStore()

const credentialOptions: Array<{ id: number; name: string }> = [
  { id: 1, name: '叉车司机N1证' },
  { id: 2, name: '低压电工证' },
  { id: 4, name: '工程机械维修工·五级' },
  { id: 5, name: '工程机械维修工·四级' }
]

const papers = ref<Paper[]>([
  { id: 101, year: 2024, title: '2024年叉车司机N1真题（A卷）', question_count: 100, duration_minutes: 90, credential_id: 1, credential_name: '叉车司机N1证', source: '应急管理局', difficulty: '进阶' },
  { id: 102, year: 2024, title: '2024年叉车司机N1真题（B卷）', question_count: 100, duration_minutes: 90, credential_id: 1, credential_name: '叉车司机N1证', source: '应急管理局', difficulty: '进阶' },
  { id: 103, year: 2023, title: '2023年叉车司机N1真题', question_count: 80, duration_minutes: 60, credential_id: 1, credential_name: '叉车司机N1证', source: '应急管理局·浙江', difficulty: '入门' },
  { id: 104, year: 2023, title: '2023年叉车司机N1专项卷·液压与制动', question_count: 60, duration_minutes: 60, credential_id: 1, credential_name: '叉车司机N1证', source: '省特检院', difficulty: '专项' },
  { id: 105, year: 2022, title: '2022年叉车司机N1真题', question_count: 80, duration_minutes: 60, credential_id: 1, credential_name: '叉车司机N1证', source: '应急管理局', difficulty: '入门' },
  { id: 201, year: 2024, title: '2024年低压电工真题', question_count: 90, duration_minutes: 90, credential_id: 2, credential_name: '低压电工证', source: '应急管理局', difficulty: '进阶' },
  { id: 401, year: 2024, title: '2024年维修工五级真题·叉车维修方向', question_count: 120, duration_minutes: 120, credential_id: 4, credential_name: '工程机械维修工·五级', source: '人社部', difficulty: '认证' },
  { id: 402, year: 2023, title: '2023年维修工五级真题', question_count: 100, duration_minutes: 90, credential_id: 4, credential_name: '工程机械维修工·五级', source: '人社部·华东片区', difficulty: '专项' }
])

const route = useRoute()
const variant = computed(() => {
  const v = String(route.query.variant || 'a').toLowerCase()
  return ['a', 'b', 'c'].includes(v) ? v : 'a'
})

const switcherVariants = [
  { key: 'a', label: '紧凑卡片网格' },
  { key: 'b', label: '列表+筛选头' },
  { key: 'c', label: '年份分组时间线' }
]

const filterCredentialId = ref<number>(1)
const filterYear = ref<number | null>(null)
const filterDifficulty = ref<Difficulty | null>(null)
const keyword = ref('')
const selectedId = ref<number | null>(null)

watch(
  () => credentialStore.current?.id,
  (id) => {
    if (id && credentialOptions.some(c => c.id === id)) filterCredentialId.value = id
  },
  { immediate: true }
)

const currentCredentialName = computed(() => {
  const opt = credentialOptions.find(c => c.id === filterCredentialId.value)
  return opt?.name || credentialStore.current?.name || '叉车司机N1证'
})

const yearOptions = computed(() => {
  const years = [...new Set(papers.value.map(p => p.year))].sort((a, b) => b - a)
  return years
})

const filteredPapers = computed(() => {
  return papers.value.filter(p => {
    if (p.credential_id !== filterCredentialId.value) return false
    if (filterYear.value && p.year !== filterYear.value) return false
    if (filterDifficulty.value && p.difficulty !== filterDifficulty.value) return false
    if (keyword.value.trim()) {
      const kw = keyword.value.trim().toLowerCase()
      if (!p.title.toLowerCase().includes(kw) && !p.source.toLowerCase().includes(kw)) return false
    }
    return true
  })
})

const selectedPaper = computed(() => papers.value.find(p => p.id === selectedId.value) || null)

function handleSelect(id: number) {
  selectedId.value = id
  ElMessage.info('功能建设中')
}

function clearFilters() {
  filterYear.value = null
  filterDifficulty.value = null
  keyword.value = ''
}
</script>

<style scoped>
.real-exam-prototype {
  max-width: 1200px;
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
  margin-bottom: 18px;
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
</style>
