<template>
  <div class="real-exam-papers">
    <div class="real-exam-header">
      <div>
        <h2 class="real-exam-title">真题练习</h2>
        <p class="real-exam-sub">
          当前证件：<strong>{{ currentCredentialName || '—' }}</strong>
          <span class="real-exam-sub-sep">·</span>
          已展示 {{ filteredPapers.length }} / {{ papers.length }} 套
        </p>
      </div>
    </div>

    <div v-if="filteredPapers.length === 0" class="empty-wrap">
      <el-empty :description="emptyDescription" />
    </div>
    <div v-else class="variant-c-timeline">
      <div v-for="[year, list] in grouped" :key="year" class="timeline-year">
        <div class="timeline-year-head">
          <span class="timeline-dot"></span>
          <span class="timeline-year-label">{{ year }}年</span>
          <span class="timeline-year-count">{{ list.length }}套</span>
        </div>
        <div class="timeline-cards">
          <div
            v-for="p in list"
            :key="p.id"
            class="timeline-card"
            :class="{ 'is-selected': selectedId === p.id }"
            @click="handleSelect(p.id)"
          >
            <div class="timeline-card-title">{{ p.title }}</div>
            <div class="timeline-card-meta">
              <el-tag size="small" :type="levelTagType(p.difficulty)">{{ p.difficulty }}</el-tag>
              <span class="meta-text">{{ p.source }} · {{ p.question_count }}题 · {{ p.duration_minutes }}分钟</span>
            </div>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { useCredentialStore } from '@/stores/credential'
import { levelTagType } from '@/constants/level'

type Difficulty = '入门' | '进阶' | '专项' | '认证'

interface Paper {
  id: number
  year: number
  title: string
  question_count: number
  duration_minutes: number
  credential_id: number
  credential_name: string
  source: string
  difficulty: Difficulty
}

const credentialStore = useCredentialStore()

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

const currentCredentialName = computed(() => credentialStore.current?.name || '')

const filteredPapers = computed(() => {
  const cid = credentialStore.current?.id
  if (!cid) return []
  return papers.value.filter(p => p.credential_id === cid)
})

const emptyDescription = computed(() =>
  currentCredentialName.value ? `${currentCredentialName.value} 真题建设中，敬请期待` : '真题建设中，敬请期待'
)

const grouped = computed(() => {
  const map = new Map<number, Paper[]>()
  for (const p of filteredPapers.value) {
    if (!map.has(p.year)) map.set(p.year, [])
    map.get(p.year)!.push(p)
  }
  return [...map.entries()].sort((a, b) => b[0] - a[0])
})

const selectedId = ref<number | null>(null)

function handleSelect(id: number) {
  selectedId.value = id
  ElMessage.info('功能建设中')
}
</script>

<style scoped>
.real-exam-papers {
  max-width: 1200px;
  margin: 0 auto;
}
.real-exam-header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 16px;
  margin-bottom: 18px;
  flex-wrap: wrap;
}
.real-exam-title {
  font-size: var(--text-2xl);
  line-height: 1.2;
  margin: 0 0 6px;
  color: var(--color-text-primary);
}
.real-exam-sub {
  font-size: 13px;
  color: var(--color-text-tertiary);
  margin: 0;
}
.real-exam-sub-sep {
  margin: 0 6px;
  color: var(--color-text-muted);
}
.empty-wrap {
  padding: 24px 0;
  background: var(--color-bg-card);
  border: 1px solid var(--color-border-light);
  border-radius: var(--radius-lg);
}
.variant-c-timeline {
  display: flex;
  flex-direction: column;
  gap: 20px;
}
.timeline-year-head {
  display: flex;
  align-items: center;
  gap: 10px;
  margin-bottom: 10px;
}
.timeline-dot {
  width: 10px;
  height: 10px;
  border-radius: 9999px;
  background: var(--color-primary-500);
  box-shadow: 0 0 0 4px var(--color-primary-100);
  flex-shrink: 0;
}
.timeline-year-label {
  font-size: 16px;
  font-weight: 700;
  color: var(--color-text-primary);
}
.timeline-year-count {
  font-size: 12px;
  color: var(--color-text-muted);
  background: var(--color-bg-page);
  border: 1px solid var(--color-border-light);
  padding: 2px 8px;
  border-radius: var(--radius-full);
}
.timeline-cards {
  margin-left: 20px;
  padding-left: 16px;
  border-left: 1px dashed var(--color-border);
  display: flex;
  flex-direction: column;
  gap: 10px;
}
.timeline-card {
  padding: 12px 14px;
  background: var(--color-bg-card);
  border: 1px solid var(--color-border-light);
  border-radius: var(--radius-lg);
  cursor: pointer;
  transition: border-color 150ms var(--ease-default), box-shadow 150ms var(--ease-default);
}
.timeline-card:hover {
  border-color: var(--color-border);
}
.timeline-card.is-selected {
  border-color: var(--color-primary-200);
  box-shadow: 0 0 0 2px var(--color-primary-100);
}
.timeline-card-title {
  font-size: 14px;
  font-weight: 600;
  color: var(--color-text-primary);
  margin-bottom: 8px;
}
.timeline-card-meta {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}
.meta-text {
  font-size: 12px;
  color: var(--color-text-tertiary);
}
</style>
