// useQuestionPeripherals：答题外围交互 module（收藏 / 知识点 / 作答计时）。
// 会话本职（题目推进、判定、断点）在 usePracticeSession；收藏、知识点卡、题计时是
// 挂在会话上的三件外围交互——由此 module 统一承接，页面从「自建 watch」变「声明要哪几件」。
// deep module：小 interface（三个可选 adapter）藏大量实现：
// - 进题时机统一挂 session.onQuestionEnter 钩子（重置各自状态后拉取），按 duration →
//   favorite → knowledge 注册序触发（与原页内合并 watch 的执行序一致）；
// - knowledge 两种触发形态：'result' 出结果后按结果题目查（题库练习），'enter' 进题预取
//   （真题练习）——两页既有的查询触发时机不同，由 trigger 如实保留；
// - 作答计时纯本地（questionStartTime/lastDuration）：进题起表，页面在提交前调
//   recordDuration() 取用时秒数。
// adapter 只做 API 绑定（允许抛错），失败降级由本 module 统一：收藏查询失败降级为未收藏、
// 知识点查询失败置空，均不阻断答题。未注入的交互为 no-op（favorited 恒 false、
// knowledgeTags 恒空、lastDuration 恒 undefined）。
// composable 本体不 import 任何 api；question 域的标准绑定由文件尾的
// questionPeripheralAdapters() 工厂提供（两页一行消费，需要差异的页面可逐项覆盖）。
import { ref, watch } from 'vue'
import type { Ref } from 'vue'
import type { Question, SubmitResult } from '@/types/question'
import { favoriteApi } from '@/api/favorite'
import { questionInteractionApi } from '@/api/questionInteraction'

/** 外围 module 依赖的会话面（usePracticeSession 返回值的子集） */
export interface QuestionPeripheralsSession {
  currentQuestion: Ref<Question | null>
  lastResult: Ref<SubmitResult | null>
  onQuestionEnter: (cb: (q: Question | null) => void) => () => void
}

/** 收藏状态查询响应（与 favoriteApi.check 响应同形，adapter 透传） */
export interface PeripheralFavoriteCheck {
  favorited: boolean
  favorite_id?: number | null
}

/** 收藏 adapter：题目域收藏 API 绑定（target_type='question' 由 adapter 固定） */
export interface FavoritePeripheralAdapter {
  check(questionId: number): Promise<PeripheralFavoriteCheck>
  add(questionId: number): Promise<{ favorite_id?: number | null }>
  remove(favoriteId: number): Promise<unknown>
}

/** 知识点触发形态：'result' 出结果后查（题库练习）；'enter' 进题预取（真题练习） */
export type KnowledgeTrigger = 'enter' | 'result'

/** 知识点 adapter：题目知识点标签查询 */
export interface KnowledgePeripheralAdapter {
  trigger: KnowledgeTrigger
  list(questionId: number): Promise<any[]>
}

/** 外围 adapter 集：三件交互各自可选 */
export interface QuestionPeripheralsAdapters {
  /** 进题查收藏状态 + 切换收藏 */
  favorite?: FavoritePeripheralAdapter
  /** 查知识点卡（trigger 决定挂进题还是出结果时机） */
  knowledge?: KnowledgePeripheralAdapter
  /** 作答计时为纯本地状态，无 API 可注入；置 true 启用 */
  duration?: boolean
}

/** 收藏切换结果：'added'/'removed' 成功；null = 无当前题/未注入 adapter/失败（页面据此决定提示） */
export type FavoriteToggleResult = 'added' | 'removed' | null

/**
 * 答题外围交互：按需声明收藏/知识点/作答计时。
 * 返回 favorited / toggleFavorite / knowledgeTags / lastDuration / recordDuration，
 * 命名与练习页、真题页既有模板绑定一致，页面模板零改动。
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

  // ===== 收藏（进题查状态 + 切换）=====
  const favorited = ref(false)
  const favoriteId = ref(0)

  const favorite = adapters.favorite
  if (favorite) {
    session.onQuestionEnter(async (q) => {
      favorited.value = false
      favoriteId.value = 0
      if (!q) return
      try {
        const res = await favorite.check(q.id)
        favorited.value = !!res?.favorited
        favoriteId.value = res?.favorite_id || 0
      } catch {
        // 查询失败降级为未收藏
      }
    })
  }

  /** 切换当前题收藏；结果供页面决定提示文案（未注入 adapter / 失败 / 无当前题返回 null） */
  async function toggleFavorite(): Promise<FavoriteToggleResult> {
    const q = session.currentQuestion.value
    if (!q || !favorite) return null
    try {
      if (favorited.value && favoriteId.value) {
        await favorite.remove(favoriteId.value)
        favorited.value = false
        favoriteId.value = 0
        return 'removed'
      }
      const res = await favorite.add(q.id)
      favorited.value = true
      favoriteId.value = res?.favorite_id || 0
      return 'added'
    } catch {
      // 收藏/取消失败静默（错误已由拦截器提示）
      return null
    }
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
    // 收藏
    favorited,
    toggleFavorite,
    // 知识点
    knowledgeTags,
    // 作答计时
    lastDuration,
    recordDuration
  }
}

/**
 * question 域默认外围 adapter 工厂：绑定 favoriteApi / questionInteractionApi 的标准实现
 * （收藏/知识点/计时三件齐发，#617 错题重做等后续接入复用此唯一绑定点）。
 * 仅知识点触发时机因页面而异，由 knowledgeTrigger 声明（默认 'result' 出结果查）；
 * 其余需要差异的页面可对返回值逐项覆盖（注入 seam 保留在 useQuestionPeripherals）。
 */
export function questionPeripheralAdapters(
  options: { knowledgeTrigger?: KnowledgeTrigger } = {}
): QuestionPeripheralsAdapters {
  return {
    favorite: {
      check: (qid) => favoriteApi.check({ target_type: 'question', target_id: qid }),
      add: (qid) => favoriteApi.add({ target_type: 'question', target_id: qid }),
      remove: (favoriteId) => favoriteApi.remove(favoriteId)
    },
    knowledge: {
      trigger: options.knowledgeTrigger ?? 'result',
      list: (qid) => questionInteractionApi.listKnowledge(qid)
    },
    duration: true
  }
}
