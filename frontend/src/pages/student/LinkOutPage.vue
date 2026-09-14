<!--
  「即将离开本站」中转页（#881 / ADR-0044）——**独立页面**。

  为什么需要它：正文里的站外链接是唯一一处内容能把读者带离本站的地方。
  过一道确认既让读者知道要去哪里、可随时返回，也让平台对导流有可见的告知。

  为什么独立（不挂布局外壳）：它是一次性的确认页 —— 读者到这里只为看一眼目标地址，
  随即要么返回、要么继续访问，不该被侧栏、导航与主题入口拖着。故本页在路由表里是
  **顶层记录**（不写 layout，见 config/pages.ts），页壳由自己撑满视口并居中。

  主题靠**继承，不放切换入口**：主题态由全局 theme store 落在 <html>（data-theme + .dark，
  首屏另有 index.html 内联脚本），本页只用设计 token（bg-canvas / bg-panel / text-ink…），
  浅色与深色都自动跟随。ui-conventions「新增任何独立布局时，必须一并评估主题入口」的
  评估结论在此是**有意不放**：本页不是布局（无 layout 键），且读者是从站内一跳而来，
  切换入口就在来处；放一个按钮反而与「一次确认、随即离开」的页面语义相冲。

  安全要点：url 来自 query，是**用户可控输入**。
    - 只接受绝对 http(s) 地址，其余一律当作无效（挡掉 javascript: / data: 等伪协议）；
    - 展示时用文本插值（双大括号）而不是 v-html —— 目标地址本身就是不可信输入，
      若按 HTML 渲染，中转页会变成第二个注入面；
    - 「继续访问」是普通 a 标签，靠 rel=noopener noreferrer nofollow 隔离，
      不用脚本 window.open（少一处需要自己维护的安全属性）。
-->
<script setup lang="ts">
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
  <div class="flex min-h-screen items-center justify-center bg-canvas p-6">
    <!-- 独立页壳：自己撑满视口并居中，不依赖任何布局组件。
         ⚠️ 模板只允许这一个根节点：根之上再放注释或兄弟节点会让组件变成 fragment
         （attrs 透传与 w.element 都会静默错位，ForumContent 已踩过一次）。 -->
    <div class="w-full max-w-[520px] rounded-card border border-line bg-panel p-6 text-center shadow-card">
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
