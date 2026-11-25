<template>
  <div class="flex flex-col gap-6">
    <UiPageHeader title="工作台" class="mb-0!" />

    <UiCard v-for="section in shortcutSections" :key="section.title">
      <UiSectionHeader :title="section.title" />
      <ul class="grid list-none grid-cols-1 gap-2 p-0 sm:grid-cols-2 xl:grid-cols-4">
        <li v-for="link in section.links" :key="link.key">
          <RouterLink
            :to="{ name: link.routeName }"
            class="flex min-h-[var(--space-10)] items-center gap-3 rounded-ctl border border-line px-3 text-sm text-ink no-underline hover:bg-canvas hover:underline"
          >
            <UiIcon v-if="typeof link.icon === 'string'" :name="link.icon" :size="18" />
            <span>{{ link.label }}</span>
          </RouterLink>
        </li>
      </ul>
    </UiCard>

    <p v-if="!shortcutSections.length && !statisticsLink" class="text-ink-3">
      请从侧栏进入你有权限使用的页面。
    </p>

    <section v-if="statisticsLink" aria-label="统计摘要">
      <UiSectionHeader title="统计摘要">
        <template #actions>
          <RouterLink :to="{ name: statisticsLink.routeName }" class="text-sm underline">
            查看统计分析
          </RouterLink>
        </template>
      </UiSectionHeader>
      <UiAsyncSection
        :loading="loading"
        :error="loadError"
        :retrying="retrying"
        @retry="retry"
      >
        <div v-if="overview" class="grid grid-cols-1 gap-3 sm:grid-cols-2 xl:grid-cols-4">
          <UiStatCard label="学员总数" :value="overview.total_students ?? 0" tone="brand" />
          <UiStatCard label="今日活跃学员" :value="overview.active_today ?? 0" tone="ok" />
          <UiStatCard label="课程总数" :value="overview.total_courses ?? 0" tone="warn" />
          <UiStatCard
            label="总学习时长"
            :value="formatDurationCompact(overview.total_study_duration ?? 0)"
            tone="neutral"
          />
        </div>
      </UiAsyncSection>
    </section>
  </div>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { RouterLink } from 'vue-router'
import { adminApi, type AdminStatisticsOverview } from '@/api/admin'
import UiAsyncSection from '@/components/ui/UiAsyncSection.vue'
import UiCard from '@/components/ui/UiCard.vue'
import UiIcon from '@/components/ui/UiIcon.vue'
import UiPageHeader from '@/components/ui/UiPageHeader.vue'
import UiSectionHeader from '@/components/ui/UiSectionHeader.vue'
import UiStatCard from '@/components/ui/UiStatCard.vue'
import { useAsyncPage } from '@/composables/useAsyncPage'
import { adminNav, filterNavByCapability, type NavItem } from '@/config/navigation'
import type { RouteName } from '@/config/pages'
import { useAuthStore } from '@/stores/auth'
import { holdsCapability } from '@/utils/authzRuntime'
import { formatDurationCompact } from '@/utils/format'

defineOptions({ name: 'AdminDashboard' })

const authStore = useAuthStore()
const availableLinks = computed(() =>
  filterNavByCapability(adminNav, capability =>
    holdsCapability(authStore.userInfo?.role, authStore.capabilities, capability)
  ).flatMap(item => item.children ?? [item])
)

function findLink(name: RouteName): (NavItem & { routeName: RouteName }) | undefined {
  return availableLinks.value.find(
    (link): link is NavItem & { routeName: RouteName } => link.routeName === name
  )
}

const sections = [
  { title: '审核处理', routes: ['ProfileReview', 'ContributionManage', 'QuestionReview', 'ChapterReview'] },
  { title: '常用管理', routes: ['HrwaiUserManage', 'CourseCatalog', 'ForumManage', 'AdminFeedbackManage'] }
] satisfies { title: string; routes: RouteName[] }[]

const shortcutSections = computed(() => sections
  .map(({ title, routes }) => ({ title, links: routes.map(findLink).filter(link => link !== undefined) }))
  .filter(section => section.links.length > 0)
)

const statisticsLink = computed(() => findLink('Statistics'))
const overview = ref<AdminStatisticsOverview>()
const { loading, loadError, retrying, run, retry } = useAsyncPage(
  () => adminApi.getStatistics(),
  {
    credentialScoped: false,
    apply(data) {
      if (statisticsLink.value) overview.value = data.overview
    }
  }
)

watch(() => !!statisticsLink.value, allowed => {
  if (allowed) void run()
  else overview.value = undefined
}, { immediate: true })
</script>
