// useQuestionPeripherals：答题外围交互 module（收藏 / 知识点 / 作答计时）。
// 会话本职（题目推进、判定、断点）在 usePracticeSession；收藏、知识点卡、题计时是
// 挂在会话上的三件外围交互——由此 module 统一承接，页面从「自建 watch」变「声明要哪几件」。
// deep module：小 interface（两个可选 adapter + 一个 duration 开关）藏大量实现：
// - 进题时机统一挂 session.onQuestionEnter 钩子（重置各自状态后拉取），按 duration →
//   favorite → knowledge 注册序触发（与原页内合并 watch 的执行序一致）；
// - knowledge 两种触发形态：'result' 出结果后按结果题目查（题库练习），'enter' 进题预取
//   （真题练习）——两页既有的查询触发时机不同，由 trigger 如实保留；
// - 作答计时纯本地（questionStartTime/lastDuration）：进题起表，页面在提交前调
//   recordDuration() 取用时秒数。
// 收藏这一件**委托** useFavorite（ADR-0060 决策 3：查询—切换—提示—失败保持原态的实现全站只有
// 一份，target_type 由内容对象表给出），本 module 只做两件事：把进题时机接到它的 load() 上，
// 以及把它的返回值翻回页面既有的 'added'/'removed'/null 口径（提示留给页面：练习页与真题页的
// 文案口径本就不同，故 notify=false）。收藏不再有可注入的 adapter——迁移前那个
// FavoritePeripheralAdapter 只有一份实现（questionPeripheralAdapters 里的绑定），绑定点收进
// useFavorite 后它就是假想 seam（ADR-0060 理由段的 one-adapter 判据）。
// adapter 只做 API 绑定（允许抛错），失败降级由本 module 统一：知识点查询失败置空，不阻断
// 答题（收藏那件的降级口径同样只有一处实现，在 useFavorite 内：查询失败降级为未收藏、
// 切换失败保持原态）。未注入/未启用的交互为 no-op（knowledgeTags 恒空、lastDuration 恒
// undefined；收藏恒可用，无当前题时 load 不查、toggle 返回 null）。
// composable 本体不 import 任何 api；question 域的知识卡绑定由文件尾的
// questionPeripheralAdapters() 工厂提供（两页一行消费，需要差异的页面可逐项覆盖）。
import { ref, watch } from 'vue'
import type { Ref } from 'vue'
import type { Question, SubmitResult } from '@/types/question'
import { questionInteractionApi } from '@/api/questionInteraction'
import { useFavorite } from './useFavorite'

/** 外围 module 依赖的会话面（usePracticeSession 返回值的子集） */
export interface QuestionPeripheralsSession {
  currentQuestion: Ref<Question | null>
  lastResult: Ref<SubmitResult | null>
  onQuestionEnter: (cb: (q: Question | null) => void) => () => void
}

/** 知识点触发形态：'result' 出结果后查（题库练习）；'enter' 进题预取（真题练习） */
export type KnowledgeTrigger = 'enter' | 'result'

/** 知识点 adapter：题目知识点标签查询 */
export interface KnowledgePeripheralAdapter {
  trigger: KnowledgeTrigger
  list(questionId: number): Promise<any[]>
}

/** 外围 adapter 集：知识点与计时各自可选（收藏无 adapter，见 useFavorite） */
export interface QuestionPeripheralsAdapters {
  /** 查知识点卡（trigger 决定挂进题还是出结果时机） */
  knowledge?: KnowledgePeripheralAdapter
  /** 作答计时为纯本地状态，无 API 可注入；置 true 启用 */
  duration?: boolean
}

/** 收藏切换结果：'added'/'removed' 成功；null = 无当前题/失败（页面据此决定提示）——useFavorite 的 FavoriteState 翻回此口径 */
export type FavoriteToggleResult = 'added' | 'removed' | null

/**
 * 答题外围交互：按需声明收藏/知识点/作答计时。
 * 返回 favorited / toggleFavorite / knowledgeTags / lastDuration / recordDuration，
 * 命名与练习页、真题页既有模板绑定一致，页面模板零改动。
 * 收藏态本身由 useFavorite 持有（此处直接透出它的 favorited ref），本 module 不复制状态。
 */
export function useQuestionPeripherals(
  session: QuestionPeripheralsSession,
  adapters: QuestionPeripheralsAdapters = {}
) {
  // ===== 作答计时（进题起表，提交前取用时）=====
  const questionStartTime = ref<number>(Date.now())
  const lastDuration = ref<number | undefined>(undefined)

  if (adapters.duration) {
    session.onQuestionEnter(() => {
      questionStartTime.value = Date.now()
      lastDuration.value = undefined
    })
  }

  /** 提交前取当前题用时（秒）；未启用 duration 时为 no-op */
  function recordDuration(): void {
    if (!adapters.duration) return
    lastDuration.value = (Date.now() - questionStartTime.value) / 1000
  }

  // ===== 收藏（委托 useFavorite：进题查状态 + 切换；提示交回页面按返回值决定）=====
  const favorite = useFavorite(
    'question',
    () => session.currentQuestion.value?.id ?? 0,
    { notify: false }
  )
  // 进题即重查（含切题/回退/退出）：useFavorite.load 先清态再查，不留上一题的状态
  session.onQuestionEnter((q) => {
    void favorite.load(q?.id)
  })

  /** 切换当前题收藏；结果供页面决定提示文案（失败或无当前题返回 null） */
  async function toggleFavorite(): Promise<FavoriteToggleResult> {
    const next = await favorite.toggle()
    if (!next) return null
    return next.favorited ? 'added' : 'removed'
  }

  // ===== 知识点（'result' 出结果查 / 'enter' 进题预取）=====
  const knowledgeTags = ref<any[]>([])
  const knowledge = adapters.knowledge
  if (knowledge) {
    if (knowledge.trigger === 'enter') {
      session.onQuestionEnter(async (q) => {
        knowledgeTags.value = []
        if (!q) return
        try {
          knowledgeTags.value = (await knowledge.list(q.id)) || []
        } catch {
          // 知识卡查询失败不阻断
        }
      })
    } else {
      // 'result'：结果到达（含断点恢复进入已答题目）后按结果题目查；无结果时清空
      watch(
        () => (session.lastResult.value as (SubmitResult & { question_id?: number }) | null)?.question_id,
        async (qid) => {
          if (!qid) {
            knowledgeTags.value = []
            return
          }
          try {
            knowledgeTags.value = (await knowledge.list(qid)) || []
          } catch {
            knowledgeTags.value = []
          }
        }
      )
    }
  }

  return {
    // 收藏（useFavorite 的状态机出口；进题查态见上面的 onQuestionEnter 注册）
    favorited: favorite.favorited,
    toggleFavorite,
    // 知识点
    knowledgeTags,
    // 作答计时
    lastDuration,
    recordDuration
  }
}

/**
 * question 域默认外围 adapter 工厂：绑定 questionInteractionApi 的标准实现
 * （知识点/计时两件齐发，#617 错题重做等后续接入复用此唯一绑定点）。
 * 仅知识点触发时机因页面而异，由 knowledgeTrigger 声明（默认 'result' 出结果查）；
 * 其余需要差异的页面可对返回值逐项覆盖（注入 seam 保留在 useQuestionPeripherals）。
 * 收藏不在此列：它的绑定与状态机都在 useFavorite（target_type 由内容对象表给出）。
 */
export function questionPeripheralAdapters(
  options: { knowledgeTrigger?: KnowledgeTrigger } = {}
): QuestionPeripheralsAdapters {
  return {
    knowledge: {
      trigger: options.knowledgeTrigger ?? 'result',
      list: (qid) => questionInteractionApi.listKnowledge(qid)
    },
    duration: true
  }
}
