/**
 * §1.6 裸 hex 白名单（ui-spec §9 机检硬指标 1 的豁免册）。
 *
 * 规则（ui-spec §1.6）：
 * - 声明位（<style> 块内）裸 hex 必须 ∈ token 值 ∪ 本白名单；
 * - 白名单逐值一行理由、注明哪个文件为什么留，不按次数自动豁免；
 * - 主色字面值不豁免（#239EDD 必须写 $primary-color）。
 *
 * 机检消费方：utils/recruiterUiContract.test.js（批1 招聘端 9 页）、
 * utils/batch2UiContract.test.js（批2 学员端 4 页，N02 白底套收口 #1605）、
 * utils/batch4UiContract.test.js（§1.6 文件级自检：白名单值 ∩ token 值 = ∅ + 死条目 = 0）。
 * 两条自检的扫描面自 #2863 起是**全层并集**（pages/** + components/** + App.uvue，style 块剥注释后比对），
 * 不再逐票往手工并集里补件——补漏一次就是一个死条目被静默判活。
 */

module.exports = {
    // 值（归一为大写 6 位）→ 理由（注明文件与为什么没有 token 可走）
    '#E7F4F9': 'pages/recruiter/home.uvue 紧迫行浅底 + job-edit.uvue chip-active 底——品牌浅青，token 表无对应',
    '#1B7BA0': 'pages/recruiter/home.uvue 紧迫行深青文字/箭头——品牌深青，token 表无对应',
    '#22A45D': 'pages/forum/components/forum-resource-panel.uvue res-title-green——资源标题装饰绿（#2861 裁：非成功语义位，不绑 $success-color）；原 recruiter tag-open/tag-approved 两处已改走 $success-color',
    '#F0FFF0': 'pages/forum/components/forum-resource-panel.uvue res-card-bg-green——资源卡装饰浅绿底（#2861 裁：非成功语义位）；原 recruiter 两处成功浅底已改走 $success-tint',
    '#FF6B35': 'pages/recruiter/resumes.uvue 薪资/筛选徽标 + resume-detail.uvue 薪资强调——强调橙，token 表无对应（$warning-color #ff9900 偏黄）',
    '#EEF3FD': 'pages/recruiter/applications.uvue tag-applied 主色浅底——$primary-tint(#ebf5ff) 近似但值不等，视觉回归风险待真机裁',
    '#E0E0E0': 'pages/recruiter/resume-detail.uvue btn-secondary 边框（批2 #1606 并入；B3 #2519 起 practice-do 已 token 化不再持此值）——$border-color(#D9D9D9) 近似但值不等，待真机裁',
    '#1565C0': 'pages/guide/choose-cert.uvue btn-confirm 渐变深端与实底兜底——主色深阶，token 表无深阶位；与 points-detail.uvue 会员卡渐变尾色（linear-gradient(#2979ff,#1565c0)）同值（先例）；#2907 起 cert-selector 的浮层完成条退役，本值只剩整页这一处承载',
    '#888888': 'pages/guide/choose-cert.uvue province-label/error-text 次要说明文字——$text-color-secondary(#666666) 偏深、$text-color-placeholder(#999999) 偏浅，中间灰无 token；收敛待灰阶 token 票裁',
    '#F0F2F5': 'pages/guide/choose-cert.uvue province-bar 胶囊底（$bg-color(#F5F5F5)/$border-color-light(#F0F0F0) 均不等，冷灰胶囊底）——待真机裁（#2524 起 resume-edit 分割线已改走 $border-color-light）',
    '#EEEEEE': 'batch2 profile 两页分割线/边框——$border-color(#D9D9D9)/$border-color-light(#f0f0f0) 之间的更浅边框，待真机裁（#2524 起 job-list 的边框已改走 token）',

    // ── 批2（#1606 学员端 14 页）新增 ──
    // '#2979FF' —— #2533（规格 #2493 批 A）起退役：受管页并集内 16 页 51 处旧主色已全部改走
    // $primary-color（脚本侧字面用 '#239EDD'），条目失去承载（§1.6 自检：白名单值必须是真实存在的
    // 存量声明位，否则就是死条目）。组件目录那部分不在本白名单射程内，由
    // utils/legacyPrimaryColorRatchetContract.test.js 的逐文件棘轮兜住（批 B #2534 / 批 C #2536 继续清）。
    // '#4A9AFF' —— #2563 起 personal-info 头像兜底改走包内默认头像图（.cell-avatar-placeholder 的浅蓝底随类删除），该值无承载，条目退役
    '#FFF6E5': 'pages/profile/personal-info.uvue 提示条浅橙底——token 表无警告浅底对应（$warning-tint 是 #FFF3E0），语义相近但值不等，待真机裁',
    // '#FBE9E7' —— #2523 起 points-detail 流水图标位按原型退役（行内不再有图标底色），该浅红底无承载，条目退役
    '#FF8F00': 'pages/forum/components/forum-topic-header.uvue 限时/状态标签橙——$warning-color(#ff9900) 近似但值不等，待真机裁',
    // '#FFA000' / '#FFD700' —— 原型 569:4532 的金币是 3D 切图本身、没有底圆，task-center 的
    // .balance-coin 渐变圆底（起色 #FFD700 / 尾色 #FFA000）随之退役，两值无承载，条目退役
    '#222222': 'batch2 ai-assistant 两页/practice-report 重文字——$text-color(#333333) 近似但值不等，历史深灰，待文字 token 收敛裁',
    '#6DB8E8': 'components/ai-chat/ai-chat-input.uvue 发送按钮浅蓝强调——AI 模块品牌色（ai-feature 诊断面板 4 处承载随 #2262 面板摘除退役，页面本身随 #2593 并入 ai-assistant 的 scope），token 表无对应',
    '#4CAF50': 'pages/forum/components/forum-reply-list.uvue reply-avatar 头像占位底——装饰绿（#2861 裁：头像底色不是成功语义位，不绑 token）；其余 22 处成功/正确语义已改走 $success-color',
    '#4FACFE': 'pages/practice/practice-report.uvue 图表/趋势蓝——品牌蓝变体，token 表无对应',
    '#BBBBBB': 'batch2 practice-report/resume-edit 占位/禁用边框——$text-disabled(#cccccc) 近似但值不等，待真机裁',
    '#EEF1F6': 'batch2 practice-report/resume-edit 卡片浅灰底——中性底，token 表无对应，待中性底 token 立项后迁',
    '#F44336': 'pages/forum/components/forum-reply-list.uvue reply-like-liked 点赞选中底/边——强调红（#2861 裁：点赞不是危险语义位，不绑 $danger-color）；其余 19 处错误/危险语义已改走 $danger-color',
    '#16A34A': 'pages/forgot-password/forgot-password.uvue 成功页扳手插画成套主色（4 处，与 #15803D/#14532D/#86EFAC 同图）——插画色组不拆件（#2861 裁）；resume-attach 上传成功那处已改走 $success-color',
    '#FAFBFD': 'pages/resume/resume-edit.uvue 输入底——$bg-color 近似但值不等，输入件视觉回归风险待真机裁（#2862 勘误：此值 ≠ 底灰七近邻的 #FAFBFC，收敛射程外，条目保留）',
    '#C8CDD8': 'pages/resume/resume-attach.uvue 文件图标底/边框——中性灰蓝，token 表无对应',
    '#F0F7FF': 'pages/practice/practice-do.uvue 选中项/考点标签浅蓝底——$primary-tint 近似但值不等，待真机裁',

    // ── 批4（#1607 N01 蓝底套 19 页）新增 ──
    '#A0C4FF': 'pages/courses/chapter-view.uvue mark-done-btn--busy 底——主操作蓝 busy 态，token 表无对应',
    '#A8C4FF': 'pages/resources/materials.uvue material-action--busy 底——主操作蓝 busy 态，token 表无对应',
    '#EEF4FF': 'pages/resources/materials.uvue material-icon-box 底——资料图标浅蓝底，$primary-tint(#ebf5ff) 近似但值不等，待真机裁',
    '#D0E4FF': 'pages/notifications/notifications.uvue 状态条边框——浅蓝边框，$primary-tint 近似但值不等，待真机裁',
    '#E2622C': 'pages/courses/course-detail.uvue 报名状态强调字色——强调橙红，token 表无对应（$warning-color 偏黄）',
    '#E5484D': 'pages/resources/my-uploads.uvue 审核驳回原因字色等 5 处危险红变体——与 $danger-color ΔE 23.2 非近似族（#2861 裁：区别子收敛）',
    '#FF6B6B': 'pages/exam/mock-exam.uvue timer-text.danger 倒计时告警红——与 $danger-color ΔE 29 非近似族，且与 .timer-text.warning #FFF3A0 成套（#2861 裁：不改）',
    '#FFF3A0': 'pages/exam/mock-exam.uvue timer-text.warning 倒计时提醒黄——$warning-color(#ff9900) 偏深，浅黄无 token 对应',
    '#FF9800': 'pages/exam/components/exam-question-card.uvue question-score 得分强调 + components/ai-chat/ai-chat-pro-sheet.uvue sheet-cta-on 主 CTA 橙底——强调橙非警告语义位（#2861 裁）；其余 4 处进行中/提示状态已改走 $warning-color',
    // '#FFCDD2' —— #2771 起 forum-detail 页头工具栏整行删除，删除入口收进「···」菜单并改走
    // $danger-color（白卡上不再需要蓝底浅红），该值无承载，条目退役
    '#E8E8E8': 'pages/forum/check-in.uvue cell-missed 过去未打卡灰瓦片——G8 日历套历史色，中性灰无 token 对应',
    '#8A8A8A': 'pages/forum/check-in.uvue num-missed 过去未打卡日数字——中灰文字，$text-color-secondary 偏深，待灰阶 token 票裁',
    '#F6B93E': 'pages/forum/check-in.uvue badge-coin 金币徽章底——金币金，token 表无对应',
    '#C9C9C9': 'pages/forum/check-in.uvue badge-coin-missed 灰金币徽章底——$text-disabled(#cccccc) 近似但值不等，待真机裁',
    '#FFD666': 'pages/forum/check-in.uvue cell-streak 连续签到黄瓦片——贴片 580:6795 金币黄，token 表无对应（$warning-color 偏橙）',
    '#F5E3B3': 'pages/forum/check-in.uvue badge-coin-faded 未到打卡日期淡金币——贴片 580:6795 低对比金币，token 表无对应',

    // ── 批5（#1608 N05 透明页头 7 页）新增 ──
    '#6ECCFC': 'pages/courses/courses.uvue category-card 渐变首色——#2518 刀3 按原型 442:1636 实测改垂直渐变（浅青蓝→深青蓝），token 表无渐变对对应（原 #60A5FA/#3B82F6 对随旧视觉一并退役）',
    '#0FA5F1': 'pages/courses/courses.uvue category-card 渐变尾色/实底兜底——与 #6ECCFC 成对，同上',
    '#8C8C8C': 'pages/profile/profile.uvue stats-label 标签灰——$text-color-placeholder(#999999) 近似但值不等，待灰阶 token 票裁',
    '#E3F0FF': 'pages/resume/resume.uvue user-card 渐变首色——用户卡浅蓝渐变对，$primary-tint(#ebf5ff) 近似但值不等，待真机裁',
    '#CFE4FF': 'pages/resume/resume.uvue user-card 渐变尾色/实底兜底——与 #E3F0FF 成对，同上',
    '#EEF1F5': 'pages/resume/resume.uvue 完善度进度条轨灰——中性轨道灰，与 #EEF1F6（批2 卡片底）用途不同且值不等，各守各点不并条',
    // #2786 起：pro-banner 随横幅删除而失去承载，其绿系三值（含已解锁底/边框/字色）
    // 与两处棕橙字色一并退役；下面三条仍由各自承载体守着。
    '#FFD9A8': 'pages/ai-assistant/ai-assistant.uvue function-free-badge 边框（限免小签）——暖色轴细边，token 表无橙系浅边框（原与 pro-banner 共用，横幅已随 #2786 删除）',
    '#8A4B00': 'pages/ai-assistant/ai-assistant.uvue function-free-badge-text 字色（限免小签）——深橙文字，token 表无对应（原与 pro-banner 共用，横幅已随 #2786 删除）',
    '#2E9E5B': 'components/ai-chat/ai-chat-pro-sheet.uvue 已解锁按钮底——绿系按钮实底（#920 设计），与 $success-color ΔE 16.4 非近似族（#2861 裁：区别子收敛）',

    // ── 批6（#1609 N06 无页头白名单 6 页 + mall N03 补漏）新增 ──
    // '#0066FF'/'#00CC99' 已随 #2558 收敛为 $brand-grad-start/$brand-grad-end（#2516 B0 落 token）：
    // 值成了 token 值 ⇒ 白名单行必须删（§1.6 自检：白名单值不得与 token 值重叠），index.uvue 裸字面同步清零。
    '#22C55E': 'pages/index/index.uvue cta-wechat 微信品牌绿（第三方品牌色，非本端语义位）+ forgot-password 扳手插画成套（2 处）——#2861 裁：品牌色与插画组都不绑 $success-color',
    '#F2F4F7': 'pages/index/index.uvue cta-secondary 底——次级按钮中性浅灰，$bg-color(#F5F5F5) 近似但值不等，待真机裁',
    '#1E9FFF': 'pages/index/index.uvue cta-secondary-text 字色——uView 系蓝文字，token 表无对应，待蓝系 token 票裁',
    '#E0E2EA': 'pages/login/login.uvue、pages/register/register.uvue、pages/forgot-password/forgot-password.uvue 区号与输入分隔竖线——中性灰蓝分隔，$border-color(#D9D9D9) 近似但值不等，待灰阶 token 票裁',
    '#15803D': 'pages/forgot-password/forgot-password.uvue 成功态按钮按下底——#22C55E 的深一档配对（green-700），同上待成功系 token 票裁',
    '#14532D': 'pages/forgot-password/forgot-password.uvue 成功态深绿字——green-900，token 表无对应',
    '#86EFAC': 'pages/forgot-password/forgot-password.uvue 成功态浅绿字——green-300，token 表无对应',
    // '#FF4D4F' 已随 #2518 第 4 刀退役（mall 价格红改两态制 + VIP 套餐位清退，两处承载均不存在）。
    // 免费价红的新承载（product-price-free）实测核心值 #FF2424 = 既有 $color-price token 值，
    // 值 ∈ token 值 ⇒ 走 token、不进白名单（§1.6 自检：白名单值不得与 token 值重叠）。
    // '#1A1A1A' 已于 #2352 收敛为 $text-strong（#2338 裁定：新增 token，值来自现状 .md-heading 裸 hex）：
    // 值成了 token 值 ⇒ 白名单行必须删（§1.6 自检：白名单值不得与 token 值重叠）。裸字面仍散在 4 处
    // （courses.uvue ×2 / job-list.uvue / resume.uvue），渲染面迁移随 #2353–#2356；
    // 声明位判据不再拦它们（值 ∈ token 值）。
    // #2354 已把 ai-chat-bubble.uvue 从这份清单里划掉：气泡的 .md-* 整段删除、正文块改由
    // components/markdown/markdown-blocks.uvue 渲染，该文件里的 #1a1a1a 随之消失。
}
