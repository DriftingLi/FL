// 已迁移模块：响应类型**不再手写**，唯一事实源是后端注解 → backend/docs/swagger.json →
// `cd backend && go run ./cmd/gen-apitypes`（ADR-0048 决策 1/3，issue #964 片六）。
import { unwrappedRequest } from './request'
import type { WithUIQuestions } from '@/types/question'
import type { MockExamStartDTO, PracticeStartResultDTO, RealExamPaperDTO, RedeemResult } from './generated/realExam'

export type { MockExamStartDTO, PracticeStartResultDTO, RealExamPaperDTO, RedeemResult }

/** 真题套卷列表项（与后端 RealExamPaperDTO 对齐）—— 旧名保留为生成别名 */
export type RealExamPaper = RealExamPaperDTO

// questions 元素是跨域共享 UI 模型 Question（枚举窄化无法由注解表达，见 questionBank.ts 的边界说明），
// 故这两个响应用 WithUIQuestions 显式标注元素替换点，其余字段全部走生成类型。
/** 按卷练习开始/续练（与后端 PracticeStartResultDTO 对齐） */
export type RealExamPracticeStart = WithUIQuestions<PracticeStartResultDTO>

/** 按卷开考（与后端 MockExamStartDTO 对齐，后续复用 mock-exam 端点） */
export type RealExamStartResult = WithUIQuestions<MockExamStartDTO>

/** 兑换结果（与后端 RedeemResult 对齐）—— 旧名保留为生成别名 */
export type RealExamRedeemResult = RedeemResult

// 真题套卷接口，对应后端 /api/real-exam
export const realExamApi = {
  /** 套卷列表：附兑换状态与单价；证件分区走服务端 CredentialScoped 兜底（本组 JWT + 学员角色 ⇒ 不传
   *  即按登录学员当前证件；非学员/匿名不兜底，按不分区处理）。 */
  listPapers() {
    return unwrappedRequest.get<RealExamPaperDTO[]>('/real-exam/papers', { params: {} })
  },
  // 按卷练习开始/续练（未兑换时后端拒绝）
  startPractice(paperId: number) {
    return unwrappedRequest.get<RealExamPracticeStart>(`/real-exam/papers/${paperId}/practice`)
  },
  // 按卷开考：返回 mock_exam_id，之后复用 mock-exam 的 save/submit/result 端点
  startExam(paperId: number) {
    return unwrappedRequest.post<RealExamStartResult>(`/real-exam/papers/${paperId}/exam`)
  },
  // 积分兑换单套卷（重复兑换后端报"已兑换"）
  redeemPaper(paperId: number) {
    return unwrappedRequest.post<RedeemResult>(`/real-exam/papers/${paperId}/redeem`)
  }
}
