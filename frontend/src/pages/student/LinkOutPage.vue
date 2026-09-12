<script setup lang="ts">
/**
 * 「即将离开本站」中转页（#881 / ADR-0044）。
 *
 * 为什么需要它：正文里的站外链接是**唯一一处内容能把读者带离本站**的地方。
 * 过一道确认既让读者知道要去哪里、可随时返回，也让平台对导流有可见的告知。
 *
 * 安全要点：url 来自 query，是**用户可控输入**。
 *   - 只接受绝对 http(s) 地址，其余一律当作无效（挡掉 javascript: / data: 等伪协议）；
 *   - 展示时用文本插值（双大括号）而不是 v-html —— 目标地址本身就是不可信输入，
 *     若按 HTML 渲染，中转页会变成第二个注入面；
 *   - 「继续访问」是普通 a 标签，靠 rel=noopener noreferrer nofollow 隔离，
 *     不用脚本 window.open（少一处需要自己维护的安全属性）。
 */
import { computed } from "vue"
import { useRoute, useRouter } from "vue-router"
import UiButton from "@/components/ui/UiButton.vue"

const route = useRoute()
const router = useRouter()

/** 目标地址：仅接受绝对 http(s) 地址，其余视为无效 */
const targetUrl = computed(() => {
  const raw = String(route.query.url ?? "")
  return /^https?:[/][/]/i.test(raw) ? raw : ""
})

function goBack() {
  if (window.history.length > 1) router.back()
  else router.push({ name: "ForumPage" })
}
</script>

<template>
  <div class="mx-auto flex max-w-[520px] flex-col items-center px-4 pt-16">
    <div class="w-full rounded-card bg-panel p-6 text-center shadow-card">
      <h1 class="m-0 text-lg font-semibold text-ink">即将离开本站</h1>

      <template v-if="targetUrl">
        <p class="mt-2 mb-0 text-sm text-ink-2">你将要访问的是站外地址：</p>
        <!-- 目标地址是不可信输入：按文本渲染，不解析为 HTML -->
        <p class="mt-3 mb-0 break-all rounded-[6px] border border-line bg-canvas px-3 py-2 text-left text-[13px] text-ink-2">
          {{ targetUrl }}
        </p>
        <p class="mt-3 mb-0 text-xs text-ink-3">站外内容由第三方提供，请自行判断其可靠性。</p>

        <div class="mt-5 flex items-center justify-center gap-3">
          <UiButton @click="goBack">返回</UiButton>
          <a
            :href="targetUrl"
            target="_blank"
            rel="noopener noreferrer nofollow"
            class="inline-flex items-center rounded-ctl bg-ui-500 px-4 py-2 text-sm text-white no-underline hover:bg-ui-600"
          >
            继续访问
          </a>
        </div>
      </template>

      <template v-else>
        <p class="mt-3 mb-0 text-sm text-ink-2">这个链接无效或已失效。</p>
        <div class="mt-5">
          <UiButton @click="goBack">返回</UiButton>
        </div>
      </template>
    </div>
  </div>
</template>
