import { unwrappedRequest } from './request'
import { downloadBlob, dispositionHeaderValue } from '@/composables/useReportDownload'

export type ExportKind = 'students' | 'questions' | 'evaluations'

// 后端为下载文件名唯一真值（随 Content-Disposition 下发，#230）。
// 以下仅作跨域拿不到响应头时的显式回退名，不再作为文件名来源。
const exportFallbackNames: Record<ExportKind, string> = {
  students: '学员名单.csv',
  questions: '题库.csv',
  evaluations: '评估记录.csv'
}

export async function downloadExport(kind: ExportKind) {
  // responseType: blob + raw 时拦截器返回 { data, headers }，可从响应头解析文件名
  const res = await unwrappedRequest.get<Blob>(`/admin/export/${kind}`, {
    responseType: 'blob',
    raw: true
  })
  downloadBlob(res.data, exportFallbackNames[kind], {
    disposition: dispositionHeaderValue(res.headers['content-disposition'])
  })
}