<template>
  <div class="onboarding-page" v-loading="loading">
    <div class="onboarding-card">
      <h1 class="title">选择目标证件</h1>
      <p class="subtitle">请选择您想要考取的证件，系统将为您展示对应的课程与题库</p>

      <div v-if="!grouped.special_operation.length && !grouped.skill_level.length && !loading" class="empty">
        <el-empty description="暂无证件" />
      </div>

      <template v-else>
        <section class="group">
          <h2 class="group-title">特种作业上岗证</h2>
          <div class="card-grid">
            <div
              v-for="c in grouped.special_operation"
              :key="c.id"
              class="credential-card"
              :class="{ active: selectedId === c.id }"
              @click="selectedId = c.id"
            >
              <div class="card-name">{{ c.name }}</div>
              <div class="card-desc">{{ c.description }}</div>
              <div class="card-footer">
                <span class="badge special">特种作业</span>
                <el-icon v-if="selectedId === c.id" class="check-icon"><CircleCheckFilled /></el-icon>
              </div>
            </div>
          </div>
        </section>

        <section class="group">
          <h2 class="group-title">工程机械维修工（叉车维修方向）</h2>
          <div class="card-grid">
            <div
              v-for="c in grouped.skill_level"
              :key="c.id"
              class="credential-card"
              :class="{ active: selectedId === c.id }"
              @click="selectedId = c.id"
            >
              <div class="card-name">{{ c.name }}</div>
              <div class="card-desc">{{ c.description }}</div>
              <div class="card-footer">
                <span class="badge skill">L{{ c.level }} · {{ levelText(c.level) }}</span>
                <el-icon v-if="selectedId === c.id" class="check-icon"><CircleCheckFilled /></el-icon>
              </div>
            </div>
          </div>
        </section>
      </template>

      <div class="actions">
        <el-button type="primary" size="large" :loading="submitting" :disabled="!selectedId" @click="handleConfirm">
          确定进入
        </el-button>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { useRouter } from 'vue-router'
import { CircleCheckFilled } from '@element-plus/icons-vue'
import { ElMessage } from 'element-plus'
import { useCredentialStore } from '@/stores/credential'

const router = useRouter()
const credentialStore = useCredentialStore()

const loading = ref(false)
const submitting = ref(false)
const selectedId = ref<number | null>(null)

const grouped = computed(() => credentialStore.grouped)

function levelText(level: number | null) {
  const map: Record<number, string> = { 5: '初级工', 4: '中级工', 3: '高级工', 2: '技师', 1: '高级技师' }
  return level ? map[level] || '' : ''
}

async function handleConfirm() {
  if (!selectedId.value) return
  submitting.value = true
  try {
    await credentialStore.switchTo(selectedId.value)
    ElMessage.success('设置成功')
    router.replace('/training')
  } catch (e: any) {
    ElMessage.error(e?.message || '设置失败，请重试')
  } finally {
    submitting.value = false
  }
}

onMounted(async () => {
  loading.value = true
  try {
    await credentialStore.loadGrouped()
    // 若已持有 current，则预选
    if (credentialStore.current?.id) selectedId.value = credentialStore.current.id
    else {
      // 尝试加载 current
      const cur = await credentialStore.loadCurrent().catch(() => null)
      if (cur?.id) selectedId.value = cur.id
    }
  } finally {
    loading.value = false
  }
})
</script>

<style scoped>
.onboarding-page {
  min-height: 100vh;
  display: flex;
  align-items: center;
  justify-content: center;
  background: var(--color-bg-page);
  padding: var(--space-6);
}
.onboarding-card {
  width: 100%;
  max-width: 860px;
  background: var(--color-bg-card);
  border: 1px solid var(--color-border-light);
  border-radius: var(--radius-lg);
  padding: 32px;
}
.title {
  font-size: 22px;
  font-weight: 600;
  color: var(--color-text-primary);
  margin: 0 0 8px;
  text-align: center;
}
.subtitle {
  font-size: var(--text-sm);
  color: var(--color-text-secondary);
  text-align: center;
  margin: 0 0 24px;
}
.group {
  margin-top: 20px;
}
.group-title {
  font-size: 14px;
  font-weight: 600;
  color: var(--color-text-primary);
  margin: 0 0 12px;
}
.card-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(240px, 1fr));
  gap: 12px;
}
.credential-card {
  border: 1px solid var(--color-border-light);
  border-radius: var(--radius-md);
  padding: 14px;
  cursor: pointer;
  transition: all 0.15s;
  background: white;
}
.credential-card:hover {
  border-color: var(--color-primary-300);
  background: var(--color-primary-50);
}
.credential-card.active {
  border-color: var(--color-primary-500);
  background: var(--color-primary-50);
  box-shadow: 0 0 0 2px rgba(13, 148, 136, 0.15);
}
.card-name {
  font-size: var(--text-sm);
  font-weight: 600;
  color: var(--color-text-primary);
  margin-bottom: 6px;
}
.card-desc {
  font-size: 12px;
  color: var(--color-text-tertiary);
  line-height: 1.4;
  min-height: 32px;
}
.card-footer {
  margin-top: 10px;
  display: flex;
  align-items: center;
  justify-content: space-between;
}
.badge {
  font-size: 11px;
  padding: 2px 6px;
  border-radius: var(--radius-full);
}
.badge.special {
  background: var(--color-primary-50);
  color: var(--color-primary-600);
}
.badge.skill {
  background: #ECFDF5;
  color: #059669;
}
.check-icon {
  color: var(--color-primary-500);
  font-size: 18px;
}
.actions {
  margin-top: 28px;
  display: flex;
  justify-content: center;
}
</style>
