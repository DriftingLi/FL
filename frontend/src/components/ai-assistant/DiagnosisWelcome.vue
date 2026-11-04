<template>
  <!-- 诊断欢迎区（票 2241 要素 1/2/3 的落点：品牌卡 + 六张诉求卡 + FAQ）。
       三块都挂在壳的 welcome-top / welcome-bottom 槽位里，不另起布局。
       **超出首屏是必然的**：2241 已拍板三件套全保留，溢出由壳的欢迎区自身滚动承担
       （ChatPageShell 的 welcome-area 有 max-height + overflow-y-auto），
       不能让 chat-main 滚 —— 否则矮屏下输入框被挤出视口。

       数据未就绪或上游未配置时**整块不渲染**（v-if），欢迎区退回只显示预设提示词：
       诊断服务 502 时画一堆点不动的卡片比留白更糟（票 2244 拍板）。 -->
  <div v-if="brands.length || faq.length" class="mt-5 flex flex-col gap-4 text-left">
    <!-- 品牌卡：一行两张，点卡 = 打开向导并预选品牌（不是导航） -->
    <div v-if="brands.length" class="brand-cards mx-auto grid w-full max-w-[600px] grid-cols-2 gap-2">
      <button
        v-for="b in brands"
        :key="b.id"
        type="button"
        class="brand-card flex cursor-pointer items-center gap-2.5 rounded-card border border-line bg-panel px-3.5 py-3 text-left transition-all duration-[var(--duration-fast)] ease-[var(--ease-default)] hover:border-ui-400 hover:bg-ui-soft"
        @click="emit('pick-brand', b.id)"
      >
        <span class="shrink-0 text-xl leading-none" aria-hidden="true">{{ b.icon }}</span>
        <span class="min-w-0">
          <span class="block truncate text-sm font-semibold text-ink">{{ b.name }}</span>
          <span class="block truncate text-xs leading-[1.5] text-ink-3">{{ b.tagline }}</span>
        </span>
      </button>
    </div>

    <!-- 六张诉求卡：**硬编码**，不从上文推导（v2 的 faults 以 brand 为必填，没有跨品牌现象清单）。
         名字是对方语料的副本（`guide.js:83-90` 的 SCENES），措辞必须撞中语料才检索得到 ——
         联调时要拿 v2 真实 `/faults` 返回复核，对方改语料后表现为「点了有回答但答非所问」，不报错。 -->
    <div class="scenario-cards mx-auto grid w-full max-w-[600px] grid-cols-3 gap-2">
      <button
        v-for="s in SCENES"
        :key="s.fault"
        type="button"
        class="scenario-card flex cursor-pointer flex-col items-center gap-1 rounded-ctl border border-line bg-panel px-2 py-2.5 text-center transition-all duration-[var(--duration-fast)] ease-[var(--ease-default)] hover:border-ui-400 hover:bg-ui-soft"
        @click="emit('pick-scene', s.fault)"
      >
        <span class="text-base leading-none" aria-hidden="true">{{ s.icon }}</span>
        <span class="text-xs leading-[1.4] text-ink-2">{{ s.name }}</span>
      </button>
    </div>
  </div>

  <!-- FAQ：数据来自 intents.faq[]（不是本地文案），点问题 = 直接问助手 -->
  <div v-if="faq.length" class="faq-list mx-auto mt-4 w-full max-w-[600px] text-left">
    <button
      v-for="f in faq"
      :key="f.q"
      type="button"
      class="faq-item group mb-1.5 flex w-full cursor-pointer items-start gap-1.5 rounded-ctl border-0 bg-transparent px-1 py-1 text-left text-sm leading-[1.6] text-ink-2 transition-colors duration-[var(--duration-fast)] ease-[var(--ease-default)] hover:text-ui-soft-ink"
      :title="f.a"
      @click="emit('ask', f.q)"
    >
      <span class="shrink-0 text-ink-3" aria-hidden="true">❓</span>
      <span class="min-w-0 flex-1">{{ f.q }}</span>
    </button>
  </div>
</template>

<script setup lang="ts">
// 欢迎区三件套：品牌卡（intents.brands）/ 六张诉求卡（硬编码）/ FAQ（intents.faq）。
// 组件只渲染与发事件，取数与面板状态都在页面（FeatureChatPage），因为它要跨三块共享。
import type { DiagnosisFAQ, DiagnosisIntentBrand } from '@/api/aiAssistant'

withDefaults(
  defineProps<{
    brands?: DiagnosisIntentBrand[]
    faq?: DiagnosisFAQ[]
  }>(),
  { brands: () => [], faq: () => [] }
)

const emit = defineEmits<{
  /** 点品牌卡：打开向导并预选该品牌 */
  (e: 'pick-brand', brandId: string): void
  /** 点诉求卡：打开向导并预选该现象（品牌未选，向导从第一步开始） */
  (e: 'pick-scene', faultName: string): void
  /** 点 FAQ：把这句问题直接发给助手 */
  (e: 'ask', question: string): void
}>()

/**
 * 六张高频诉求卡。`fault` 必须与对方第三层选项名**逐字一致**（`guide.js:84-89`），
 * 因为这句话会进检索语料；`name` 才是给人看的短标签。
 * 不从上文推导的理由见模板注释。
 */
const SCENES = [
  { icon: '🚨', name: '仪表报警亮了', fault: '仪表报故障码' },
  { icon: '🔋', name: '整车没电', fault: '整车无法启动' },
  { icon: '🛢️', name: '举升无力', fault: '举升无力 / 不能起升' },
  { icon: '🔌', name: '充电异常', fault: '不能充电 / 充电异常' },
  { icon: '🚛', name: '不能行走', fault: '不行走 / 无法行走' },
  { icon: '🛞', name: '转向失灵', fault: '转向失灵 / 无转向' }
] as const
</script>
