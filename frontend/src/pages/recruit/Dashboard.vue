<template>
  <div class="flex flex-col gap-6">
    <section class="rounded-card border border-line bg-panel p-6 sm:p-8">
      <h1 class="font-heading text-2xl font-bold text-ink">欢迎回来，{{ userName }}！</h1>
      <p class="mt-1 text-sm text-ink-2">企业招聘工作台 · 浏览学员简历，发现合适人才</p>
    </section>

    <div class="grid gap-4 sm:grid-cols-2">
      <div class="rounded-card border border-line bg-panel p-5">
        <h2 class="text-sm font-semibold text-ink">简历库</h2>
        <p class="mt-1 text-sm text-ink-3">查看已公开的学员简历</p>
        <router-link
          to="/recruit/resumes"
          class="mt-3 inline-flex items-center gap-1 rounded-ctl bg-ui-500 px-4 py-2 text-sm font-medium text-white transition-colors hover:bg-ui-600"
        >
          进入简历库
        </router-link>
      </div>
      <div class="rounded-card border border-line bg-panel p-5">
        <h2 class="text-sm font-semibold text-ink">账号信息</h2>
        <p class="mt-1 text-sm text-ink-3">当前角色：企业招聘者</p>
        <p class="text-sm text-ink-3">账号：{{ account }}</p>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useAuthStore } from '@/stores/auth'
import { displayNameOf } from '@/types/user'

const authStore = useAuthStore()
const userName = computed(() => displayNameOf(authStore.userInfo) || authStore.userInfo?.account || '企业用户')
const account = computed(() => authStore.userInfo?.account || '-')
</script>
