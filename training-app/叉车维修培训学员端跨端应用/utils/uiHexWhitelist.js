/**
 * §1.6 裸 hex 白名单（ui-spec §9 机检硬指标 1 的豁免册）。
 *
 * 规则（ui-spec §1.6）：
 * - 声明位（<style> 块内）裸 hex 必须 ∈ token 值 ∪ 本白名单；
 * - 白名单逐值一行理由、注明哪个文件为什么留，不按次数自动豁免；
 * - 主色字面值不豁免（#239EDD 必须写 $primary-color）。
 *
 * 机检消费方：utils/recruiterUiContract.test.js（批1 招聘端 9 页）。
 * 后续批（2–6）铺开时把 pages/** 全量纳入扫描面，白名单跟着扩。
 */

module.exports = {
    // 值（归一为大写 6 位）→ 理由（注明文件与为什么没有 token 可走）
    '#E7F4F9': 'pages/recruiter/home.uvue 紧迫行浅底 + job-edit.uvue chip-active 底——品牌浅青，token 表无对应',
    '#1B7BA0': 'pages/recruiter/home.uvue 紧迫行深青文字/箭头——品牌深青，token 表无对应',
    '#22A45D': 'pages/recruiter/jobs.uvue tag-open + contacts.uvue tag-approved——成功绿；$success-color(#4cd964) 偏荧光，语义不符，收敛待 token 票裁',
    '#F0FFF0': 'pages/recruiter/jobs.uvue tag-open 底 + contacts.uvue tag-approved 底——成功绿浅底，token 表无对应',
    '#F5F5F5': 'pages/recruiter/jobs.uvue/contacts.uvue/applications.uvue tag-closed/unknown/applied-withdrawn 中性底——值虽等于 $gradient-end(#F5F5F5) 但那是渐变尾色，语义不是中性底色；待中性底 token 立项后迁',
    '#FF6B35': 'pages/recruiter/resumes.uvue 薪资/筛选徽标 + resume-detail.uvue 薪资强调——强调橙，token 表无对应（$warning-color #ff9900 偏黄）',
    '#EEF3FD': 'pages/recruiter/applications.uvue tag-applied 主色浅底——$primary-tint(#ebf5ff) 近似但值不等，视觉回归风险待真机裁',
    '#F5F6FA': 'pages/recruiter/resume-detail.uvue btn-secondary 灰底——页面局部次要按钮底，$bg-color(#F8F8F8) 近似但值不等，待真机裁',
    '#E0E0E0': 'pages/recruiter/resume-detail.uvue btn-secondary 边框——$border-color(#e5e5e5) 近似但值不等，待真机裁',
    '#F7F8FA': 'pages/recruiter/job-edit.uvue field-input/field-textarea/chip 输入底——$bg-color(#F8F8F8) 近似但值不等，输入件视觉回归风险待真机裁',
}
