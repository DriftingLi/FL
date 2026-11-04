<template>
  <!-- 四步引导向导（票 2241 要素 5：形态换、位置不换 —— 仍是 input-toolbar 的 UiCollapse，
       不改成对方那种盖住聊天区的浮层；我方壳没有浮层槽位，且林德 87 个型号在浮层里会盖住正文）。
       步骤严格按对方的顺序：品牌 → 系统/型号 → 现象/主题 → 确认并发送。 -->
  <div class="guide-body flex flex-col gap-2.5 px-0.5 pb-1 pt-1">
    <!-- 步骤条：四格，已完成的可点回去改（对方只给「← 上一步」，这里给到任意已完成步） -->
    <div class="guide-steps flex items-center gap-1 text-xs">
      <template v-for="(s, i) in STEPS" :key="s.key">
        <!-- 分隔符是纯装饰：用 span 而不是 button（button 即便 aria-hidden + tabindex=-1
             在可访问性扫描里仍是一个缺名字的控件，见 utils/__tests__/a11yIconLabels.spec.ts） -->
        <span v-if="i > 0" class="text-ink-3 opacity-60" aria-hidden="true">›</span>
        <button
          type="button"
          class="rounded-pill px-2 py-0.5 transition-colors duration-[var(--duration-fast)] ease-[var(--ease-default)]"
          :class="stepButtonClass(i)"
          :disabled="!canJumpTo(i)"
          @click="jumpTo(i)"
        >{{ i + 1 }}. {{ s.label }}</button>
      </template>
      <span class="ml-auto flex items-center gap-2">
        <button
          v-if="hasSelection"
          type="button"
          class="cursor-pointer border-0 bg-transparent p-0 text-xs text-ink-3 hover:text-ink"
          @click="reset"
        >↺ 重置</button>
      </span>
    </div>

    <!-- 第 1 步：品牌 -->
    <div v-if="step === 0" class="flex flex-wrap gap-1.5">
      <button
        v-for="b in brands"
        :key="b.id"
        type="button"
        class="flex cursor-pointer items-center gap-1.5 rounded-ctl border px-3 py-1.5 text-sm transition-all duration-[var(--duration-fast)] ease-[var(--ease-default)]"
        :class="chipClass(brandId === b.id)"
        @click="selectBrand(b.id)"
      >
        <span aria-hidden="true">{{ b.icon }}</span>{{ b.name }}
      </button>
      <!-- 品牌字典为空时（上游挂掉 / 未配置）不能留一个空 div：让人对着空白发愣，
           还以为是页面坏了。给一句能据以行动的退路。 -->
      <span v-if="!brands.length" class="py-2 text-xs leading-[1.6] text-ink-3">
        品牌列表暂时取不到，可以直接在输入框描述故障现象，或稍后重试。
      </span>
    </div>

    <!-- 第 2 步：系统（中力 14）/ 型号（林德 87） -->
    <template v-else-if="step === 1">
      <!-- 搜索框只在条目多时出现：中力 14 个系统不需要，林德 87 个型号是一面长墙（对方的判据是 >15） -->
      <UiInput
        v-if="secondItems.length > 15"
        v-model="secondFilter"
        size="small"
        placeholder="搜型号 / 车型，如 E-322、牵引车、拣选"
      />
      <div class="flex max-h-[220px] flex-wrap gap-1.5 overflow-y-auto">
        <button
          v-for="it in filteredSecondItems"
          :key="it.id"
          type="button"
          class="flex cursor-pointer items-center gap-1.5 rounded-ctl border px-3 py-1.5 text-sm transition-all duration-[var(--duration-fast)] ease-[var(--ease-default)]"
          :class="chipClass(secondId === it.id)"
          @click="selectSecond(it)"
        >
          <span aria-hidden="true">{{ it.icon || '📦' }}</span>{{ it.name }}
        </button>
        <span v-if="!filteredSecondItems.length" class="py-2 text-xs text-ink-3">
          {{ secondLoading ? '加载中…' : '没有匹配项' }}
        </span>
      </div>
    </template>

    <!-- 第 3 步：故障现象（中力）/ 维修部位·主题（林德） -->
    <template v-else-if="step === 2">
      <!-- 可信背书：这句取决于对方语料，我方经 v2 转发时没有资格做同等承诺，故改成「入口」措辞
           （对方原文是「以下每项都在维修资料库里有对应条目，查不到的项目不会出现在列表里」）。 -->
      <p class="m-0 text-xs leading-[1.6] text-ink-3">
        {{ isModelPicker
          ? '这些是该型号维修手册里的常见主题，也可以直接在下面补充描述。'
          : '这些是常见现象入口，也可以直接在下面描述。' }}
      </p>
      <div class="flex max-h-[220px] flex-wrap gap-1.5 overflow-y-auto">
        <button
          v-for="it in faultItems"
          :key="it.id"
          type="button"
          class="flex cursor-pointer items-center gap-1.5 rounded-ctl border px-3 py-1.5 text-sm transition-all duration-[var(--duration-fast)] ease-[var(--ease-default)]"
          :class="chipClass(faultName === it.name)"
          @click="selectFault(it.name)"
        >
          <span aria-hidden="true">{{ it.icon || '🔧' }}</span>{{ it.name }}
        </button>
        <span v-if="!faultItems.length" class="py-2 text-xs leading-[1.6] text-ink-3">
          {{ faultLoading ? '加载中…' : '这个词表里没有匹配项。' }}
        </span>
      </div>
      <!-- 词表为空时的出口（票 2280 缺陷二）：原先这里只写「可直接在下面描述」，
           而「下面」的输入框在第 4 步、进第 4 步又要先选一个现象 —— 死循环。
           给一个明写的入口：跳过词表，直接描述。 -->
      <button
        v-if="!faultLoading"
        type="button"
        class="self-start cursor-pointer rounded-pill border border-line bg-transparent px-2.5 py-0.5 text-xs text-ink-2 transition-colors duration-[var(--duration-fast)] ease-[var(--ease-default)] hover:border-ui-400 hover:text-ink"
        @click="useFreeform"
      >{{ faultItems.length ? '没有我要的，直接描述' : '直接描述故障现象' }}</button>
    </template>

    <!-- 第 4 步：确认并发送 -->
    <template v-else>
      <!-- 自由描述模式（票 2280 缺陷二）：词表里没有用户要的现象时，现象由用户自己写。
           这是「词表为空 / 不匹配」唯一能走通的路 —— 没有它，第三层空列表就是一个死循环。 -->
      <UiInput
        v-if="freeform"
        v-model="faultName"
        type="textarea"
        :rows="2"
        placeholder="描述故障现象，如：举升无力，空载也起不来"
      />
      <!-- 命中率提示：这是对方实测出来的事实（现象描述在知识库里是抖动的，故障码直问最稳），
           与接口无关，直接成立。 -->
      <p class="m-0 text-xs leading-[1.6] text-ink-3">
        {{ isModelPicker
          ? '有仪表显示的故障码或报警号？填进下面，命中率比只描述现象高得多。'
          : '有仪表显示的故障码？填进下面，命中率最高；只描述现象时偶尔会查不到。' }}
      </p>
      <UiInput
        v-model="extra"
        type="textarea"
        :rows="2"
        :placeholder="isModelPicker
          ? '补充：故障码 / 报警号、出现的时机、部位…（可不填）'
          : '补充：故障码（如 E41）、出现的时机、异响位置…（可不填）'"
      />
      <!-- 故障码体检：五态里只有 known/typo 值得提示，其余三态静默（不打扰输入） -->
      <p v-if="codeHintText" class="m-0 text-xs leading-[1.6]" :class="codeHintKnown ? 'text-ink-2' : 'text-warn'">
        {{ codeHintText }}
      </p>
      <div class="compose-preview rounded-ctl border border-line bg-canvas px-3 py-2 text-sm leading-[1.6] text-ink-2">
        {{ sentence || '选完上面几步，这里会显示将要发出的句子…' }}
      </div>
      <div class="flex items-center justify-end gap-2">
        <UiButton variant="primary" :icon="'send'" :disabled="!sentence || disabled" @click="send">
          发送
        </UiButton>
      </div>
    </template>
  </div>
</template>

<script setup lang="ts">
// 引导向导：四步点选 → 前端组句（v2 没有 compose 端点，品牌/型号只能进正文）。
// 取数走我方后端代理（`/api/ai-assistant/diagnosis/*`），浏览器不直连对方域名。
import { computed, ref, watch } from 'vue'
import UiButton from '@/components/ui/UiButton.vue'
import UiInput from '@/components/ui/UiInput.vue'
import { aiAssistantApi, type DiagnosisCatalogItem, type DiagnosisIntentBrand } from '@/api/aiAssistant'
import { composeDiagnosisSentence, pickerKindOf } from '@/utils/diagnosisCompose'

const props = withDefaults(
  defineProps<{
    brands?: DiagnosisIntentBrand[]
    /** 外部预选（欢迎区点品牌卡 / 诉求卡传进来） */
    presetBrandId?: string
    presetFault?: string
    /** 流式中禁用发送 */
    disabled?: boolean
  }>(),
  { brands: () => [], presetBrandId: '', presetFault: '', disabled: false }
)

const emit = defineEmits<{
  /** 成句后点发送：页面负责真正发出（它持有 store 与输入框） */
  (e: 'send', sentence: string): void
}>()

const STEPS = [
  { key: 'brand', label: '品牌' },
  { key: 'second', label: '系统 / 型号' },
  { key: 'fault', label: '现象 / 主题' },
  { key: 'confirm', label: '确认' }
] as const

const step = ref(0)
const brandId = ref('')
const secondId = ref('')
const secondName = ref('')
const faultName = ref('')
const extra = ref('')
const secondFilter = ref('')

const secondItems = ref<DiagnosisCatalogItem[]>([])
const faultItems = ref<DiagnosisCatalogItem[]>([])
const secondLoading = ref(false)
const faultLoading = ref(false)
/** 自由描述模式（票 2280 缺陷二）：第三层词表里没有用户要的现象时，现象由用户自己写 */
const freeform = ref(false)

const codeHintText = ref('')
const codeHintKnown = ref(false)

/**
 * 欢迎区传进来的现象（诉求卡），**等待被兑现**的那一份。
 *
 * 为什么不能直接写进 `faultName`（票 2280 缺陷一）：下方两个取数 watcher 在品牌/系统变化时
 * 会清空 `faultName`（换品牌后旧现象必须失效，这是对的），而诉求卡进来时品牌还没选 ——
 * 用户一选品牌，预选就被那次清空抹掉了。所以预选存这里，由取数 watcher 在清空之后
 * 再把它写回去；用户自己点了词表里的现象（或改用自由描述）才作废。
 */
const pendingFault = ref('')

const brand = computed(() => props.brands.find((b) => b.id === brandId.value) ?? null)
const isModelPicker = computed(() => pickerKindOf(brand.value) === 'model')
const hasSelection = computed(() => !!brandId.value || !!faultName.value)

const sentence = computed(() =>
  composeDiagnosisSentence(brand.value, secondName.value, {
    brandId: brandId.value,
    secondId: secondId.value,
    faultName: faultName.value,
    extra: extra.value
  })
)

const filteredSecondItems = computed(() => {
  const q = secondFilter.value.trim().toLowerCase()
  if (!q) return secondItems.value
  return secondItems.value.filter((it: DiagnosisCatalogItem) => it.name.toLowerCase().includes(q))
})

// ===== 第二层取数：品牌一变就重拉（换品牌后旧的系统列表必须失效）=====
watch(brandId, async (id) => {
  secondItems.value = []
  secondId.value = ''
  secondName.value = ''
  if (!id) return
  secondLoading.value = true
  try {
    const res = await aiAssistantApi.listDiagnosisSystems(id)
    secondItems.value = res?.items ?? []
  } catch {
    // 上游未配置时这里会 502：留空并让第三步的「没有可选项」文案兜底，不弹错
    secondItems.value = []
  } finally {
    secondLoading.value = false
  }
})

// ===== 第三层取数：第二层一选就重拉 =====
watch([brandId, secondId], async ([bid, sid]) => {
  faultItems.value = []
  // 清空上一轮的已选现象（换品牌/系统后旧现象必须失效），但**预选要留住**：
  // 诉求卡进来时品牌还没定，清完立刻把 pendingFault 写回去（票 2280 缺陷一）。
  faultName.value = pendingFault.value
  freeform.value = false
  if (!bid) return
  faultLoading.value = true
  try {
    const res = await aiAssistantApi.listDiagnosisFaults(bid, sid || undefined)
    faultItems.value = res?.items ?? []
  } catch {
    faultItems.value = []
  } finally {
    faultLoading.value = false
    // 词表里没有预选的那条现象时，进第 4 步就得靠自由描述（否则成句不成立、
    // 确认步永远灰着，用户看不出是为什么）。
    if (!pendingFault.value) return
    if (!faultItems.value.length) {
      freeform.value = true
      return
    }
    // 预选的现在**在词表里**：选完系统就把它定为已选并直接进确认步。
    // 这是诉求卡的完整路径（点卡 → 选品牌 → 选系统 → 确认），中间不该再要人点一次
    // 自己刚刚才在欢迎区点过的那个现象名。
    if (secondId.value && faultItems.value.some((it) => it.name === pendingFault.value)) {
      faultName.value = pendingFault.value
      if (step.value === 2) step.value = 3
    }
  }
})

// ===== 预选（欢迎区点卡进来）=====
// 必须定义在两个取数 watcher **之后**：immediate 的预选会直接改 brandId，若它先注册，
// 取数 watcher 还没存在，进来就是空列表（真机表现 = 点了品牌卡却没系统可选）。
watch(
  () => [props.presetBrandId, props.presetFault] as const,
  ([bid, fault], old) => {
    const [oldBid, oldFault] = old ?? ['', '']
    if (bid === oldBid && fault === oldFault) return
    if (fault) {
      // 诉求卡：现象已定，品牌未定 → 从第一步开始（品牌是 faults 的必填参数）。
      // 现象记进 pendingFault，由上方的取数 watcher 在清空后写回 faultName。
      pendingFault.value = fault
      faultName.value = fault
      if (bid) brandId.value = bid
      step.value = brandId.value ? 1 : 0
      return
    }
    if (bid) {
      pendingFault.value = ''
      brandId.value = bid
      step.value = 1
    }
  },
  { immediate: true }
)

// ===== 故障码体检：输入停顿 250ms 后打一次（对方同节奏，避免逐字发请求）=====
let hintTimer: ReturnType<typeof setTimeout> | null = null
watch(extra, (v) => {
  codeHintText.value = ''
  codeHintKnown.value = false
  if (hintTimer) clearTimeout(hintTimer)
  const q = (v || '').trim()
  if (q.length < 2) return
  hintTimer = setTimeout(async () => {
    try {
      const res = await aiAssistantApi.checkDiagnosisCode(q)
      // 五态里只提示两种：known = 认出来了、typo = 打错了并给了纠正建议。
      // unknown / format / none 都是「没有信息」，静默即可（对方也是这么做的）。
      if (res?.status === 'known') {
        codeHintKnown.value = true
        codeHintText.value = res.message || `已识别故障码 ${res.fix}`
      } else if (res?.status === 'typo') {
        codeHintKnown.value = false
        codeHintText.value = res.message || '故障码可能打错了'
      }
    } catch {
      // 体检失败不打扰输入
    }
  }, 250)
})

function chipClass(active: boolean) {
  return active
    ? 'border-ui-soft-ink bg-ui-soft text-ui-soft-ink font-semibold'
    : 'border-line bg-panel text-ink-2 hover:border-ui-400'
}

function stepButtonClass(i: number) {
  if (i === step.value) return 'bg-ui-soft font-semibold text-ui-soft-ink'
  return canJumpTo(i) ? 'text-ink-2 hover:bg-canvas' : 'text-ink-3 opacity-55'
}

/** 只能跳到「已完成」或当前步：未选品牌不能跳到第二步（faults 的 brand 必填） */
function canJumpTo(i: number) {
  if (i === step.value) return true
  if (i === 0) return true
  if (i === 1) return !!brandId.value
  if (i === 2) return !!brandId.value && !!secondId.value
  return !!faultName.value
}

function jumpTo(i: number) {
  if (!canJumpTo(i)) return
  step.value = i
}

function selectBrand(id: string) {
  brandId.value = id
  step.value = 1
}

function selectSecond(item: DiagnosisCatalogItem) {
  // 换系统 = 上一轮的现象作废（但欢迎区带来的预选要留着，由取数 watcher 再兑现）
  secondId.value = item.id
  secondName.value = item.name
  step.value = 2
}

function selectFault(name: string) {
  faultName.value = name
  // 用户自己点了一条：预选已被覆盖，换品牌时不该再冒出来
  pendingFault.value = ''
  freeform.value = false
  step.value = 3
}

/**
 * 跳过词表，自己描述（票 2280 缺陷二）：词表为空或不匹配时唯一的出口。
 * 现象文本由第 4 步的 textarea 写（绑同一个 faultName），成句逻辑不变。
 */
function useFreeform() {
  pendingFault.value = ''
  faultName.value = ''
  freeform.value = true
  step.value = 3
}

function reset() {
  brandId.value = ''
  secondId.value = ''
  secondName.value = ''
  faultName.value = ''
  extra.value = ''
  secondFilter.value = ''
  pendingFault.value = ''
  freeform.value = false
  step.value = 0
}

function send() {
  if (!sentence.value || props.disabled) return
  emit('send', sentence.value)
}
</script>
