# 1957 附录 · scoped 取值逐条清单

> 由 scripts/inventory-ui-forms.mjs --md 生成，**不要手改**。
> 口径：<style scoped>（含 @media 内）的声明；括号里是出现次数。

## border-radius（7 种取值 / 64 处）

| 取值 | 次数 | 落点（文件:行） |
| --- | --- | --- |
| var(--radius-md) | 36 | components/auth/AuthPageShell.vue:191<br>components/catalog/FacetItem.vue:36<br>components/credential/CredentialSwitcher.vue:208<br>components/dashboard/QuickCard.vue:142<br>components/layout/AdminTabBar.vue:152<br>components/layout/AppSidebar.vue:285<br>components/layout/AppSidebar.vue:338<br>components/layout/AppSidebarItem.vue:96<br>components/layout/AppTopBar.vue:229<br>components/layout/AppTopBar.vue:267<br>components/layout/NotificationPanel.vue:64<br>components/practice/AnswerResultCard.vue:57<br>components/student/AnsweringSessionShell.vue:146<br>components/student/AnsweringSessionShell.vue:152<br>components/student/AnsweringSessionShell.vue:157<br>components/student/DocumentViewer.vue:177<br>components/student/ForumImageGallery.vue:38<br>components/student/ImageViewer.vue:265<br>components/student/PptViewer.vue:274<br>components/student/PptViewer.vue:337<br>components/student/PptViewer.vue:402<br>components/student/QuestionOptionPicker.vue:49<br>components/student/QuestionOptionPicker.vue:56<br>components/student/VideoPlayer.vue:210<br>pages/admin/AuditLogs.vue:143<br>pages/admin/ContentGenerate.vue:386<br>pages/admin/ContentGenerate.vue:428<br>pages/admin/FeaturedContentEdit.vue:380<br>pages/admin/FeaturedContentList.vue:245<br>pages/admin/ForumManage.vue:738<br>pages/ai-assistant/FeatureChatPage.vue:512<br>pages/ai-assistant/FeatureChatPage.vue:541<br>pages/ai-assistant/FeatureChatPage.vue:588<br>pages/ai-assistant/FeatureChatPage.vue:616<br>pages/onboarding/CredentialOnboarding.vue:179<br>pages/tutor/QuestionTags.vue:350 |
| var(--radius-lg) | 15 | components/auth/AuthPageShell.vue:204<br>components/auth/AuthPageShell.vue:210<br>components/catalog/CourseCard.vue:68<br>components/catalog/FacetCard.vue:27<br>components/dashboard/QuickCard.vue:69<br>components/layout/NotificationPanel.vue:90<br>pages/admin/ContentGenerate.vue:304<br>pages/admin/CourseCatalog.vue:576<br>pages/admin/Dashboard.vue:305<br>pages/admin/Dashboard.vue:360<br>pages/admin/Dashboard.vue:376<br>pages/admin/FeaturedContentEdit.vue:370<br>pages/admin/RoleManage.vue:233<br>pages/ai-assistant/FeatureChatPage.vue:471<br>pages/onboarding/CredentialOnboarding.vue:140 |
| var(--radius-full) | 7 | components/credential/CredentialSwitcher.vue:187<br>components/dashboard/QuickCard.vue:181<br>components/layout/AdminTabBar.vue:186<br>components/layout/AppTopBar.vue:193<br>components/layout/AppTopBar.vue:304<br>pages/ai-assistant/FeatureChatPage.vue:437<br>pages/onboarding/CredentialOnboarding.vue:216 |
| 50% | 3 | components/layout/NotificationPanelBody.vue:187<br>components/student/QuestionOptionPicker.vue:54<br>pages/ai-assistant/FeatureChatPage.vue:598 |
| 0 | 1 | components/student/PptViewer.vue:426 |
| 0 var(--radius-full) var(--radius-full) 0 | 1 | components/layout/AppSidebarItem.vue:126 |
| 0 !important | 1 | components/tutor/MarkdownEditor.vue:263 |

## box-shadow（8 种取值 / 16 处）

| 取值 | 次数 | 落点（文件:行） |
| --- | --- | --- |
| var(--shadow-xs) | 5 | components/catalog/CourseCard.vue:71<br>components/dashboard/QuickCard.vue:71<br>components/student/AnsweringSessionShell.vue:146<br>pages/admin/ContentGenerate.vue:306<br>pages/admin/FeaturedContentEdit.vue:372 |
| var(--shadow-md) | 4 | components/catalog/CourseCard.vue:93<br>components/dashboard/QuickCard.vue:77<br>components/dashboard/QuickCard.vue:83<br>components/dashboard/QuickCard.vue:90 |
| var(--shadow-lg) | 2 | components/catalog/CourseCard.vue:80<br>components/student/PptViewer.vue:338 |
| none !important | 1 | components/ai-assistant/ChatPageShell.vue:823 |
| 0 0 0 1px var(--color-border) inset | 1 | components/auth/AuthPageShell.vue:212 |
| 0 0 0 1px var(--color-border-dark) inset | 1 | components/auth/AuthPageShell.vue:218 |
| 0 0 0 2px color-mix(in srgb, var(--color-primary-500) 20%, transparent) inset | 1 | components/auth/AuthPageShell.vue:223 |
| none | 1 | components/ui/UiInput.vue:105 |

## padding（72 种取值 / 126 处）

| 取值 | 次数 | 落点（文件:行） |
| --- | --- | --- |
| 20px | 11 | components/ui/UiDialog.vue:168<br>pages/admin/AISettings.vue:399<br>pages/admin/AuditLogs.vue:126<br>pages/admin/ContentGenerate.vue:284<br>pages/admin/Credentials.vue:237<br>pages/admin/FeaturedContentEdit.vue:348<br>pages/admin/FeaturedContentList.vue:217<br>pages/admin/HrwaiUserManage.vue:309<br>pages/admin/Statistics.vue:302<br>pages/admin/TutorManage.vue:263<br>pages/tutor/QuestionTags.vue:315 |
| 0 | 8 | components/ai-assistant/ChatPageShell.vue:820<br>components/catalog/FacetCard.vue:35<br>components/layout/AdminTabBar.vue:184<br>components/layout/AppSidebar.vue:362<br>components/student/SpreadsheetViewer.vue:86<br>components/ui/UiInput.vue:106<br>pages/admin/QuestionReview.vue:314<br>pages/ai-assistant/FeatureChatPage.vue:606 |
| 12px | 7 | pages/admin/AISettings.vue:472<br>pages/admin/AuditLogs.vue:141<br>pages/admin/ContentGenerate.vue:433<br>pages/admin/HrwaiUserManage.vue:328<br>pages/admin/Statistics.vue:347<br>pages/admin/TutorManage.vue:282<br>pages/tutor/QuestionTags.vue:398 |
| 16px | 6 | components/practice/AnswerResultCard.vue:58<br>components/student/PptViewer.vue:321<br>pages/admin/ContentGenerate.vue:426<br>pages/admin/ContentGenerate.vue:437<br>pages/admin/ContributionManage.vue:306<br>pages/admin/RecruiterManage.vue:347 |
| var(--space-3) var(--space-2) | 5 | components/catalog/FacetCard.vue:28<br>components/credential/CredentialSwitcher.vue:155<br>components/dashboard/QuickCard.vue:141<br>components/layout/AppSidebarItem.vue:131<br>pages/admin/CourseCatalog.vue:577 |
| var(--space-4) | 4 | components/layout/NotificationPanel.vue:92<br>layouts/SidebarLayout.vue:249<br>pages/admin/Dashboard.vue:359<br>pages/admin/RoleManage.vue:231 |
| 8px 12px | 4 | components/student/DocumentViewer.vue:186<br>components/student/ImageViewer.vue:273<br>components/student/QuestionOptionPicker.vue:56<br>pages/ai-assistant/FeatureChatPage.vue:539 |
| 0 var(--space-2) | 3 | components/layout/AppTopBar.vue:187<br>components/layout/AppTopBar.vue:265<br>components/layout/AppTopBar.vue:360 |
| 4px 14px | 2 | components/auth/AuthPageShell.vue:211<br>pages/ai-assistant/FeatureChatPage.vue:436 |
| var(--space-1) var(--space-3) | 2 | components/catalog/FacetCard.vue:41<br>components/layout/AdminTabBar.vue:150 |
| var(--space-2) var(--space-3) | 2 | components/catalog/FacetItem.vue:33<br>components/layout/AppSidebarItem.vue:95 |
| 0 var(--space-6) | 2 | components/layout/AdminTabBar.vue:133<br>components/layout/AppTopBar.vue:246 |
| var(--space-2) var(--space-2) | 2 | components/layout/AppSidebar.vue:204<br>components/layout/AppSidebar.vue:325 |
| var(--space-2) | 2 | components/layout/AppSidebar.vue:275<br>pages/admin/CourseCatalog.vue:584 |
| 8px 0 | 2 | components/practice/CommentCard.vue:97<br>components/student/PptViewer.vue:386 |
| 10px 15px | 2 | components/student/AnsweringSessionShell.vue:146<br>components/student/QuestionOptionPicker.vue:49 |
| 8px 16px | 2 | components/student/PptViewer.vue:287<br>components/student/PptViewer.vue:355 |
| 0 16px | 2 | components/student/PptViewer.vue:394<br>pages/student/HelpCenter.vue:150 |
| 8px | 2 | components/student/PptViewer.vue:473<br>components/student/PptViewer.vue:477 |
| 40px 20px | 2 | components/student/SpreadsheetViewer.vue:90<br>components/student/VideoPlayer.vue:271 |
| var(--space-6) | 2 | layouts/SidebarLayout.vue:173<br>pages/onboarding/CredentialOnboarding.vue:133 |
| 10px 12px | 2 | pages/admin/ContentGenerate.vue:385<br>pages/tutor/QuestionTags.vue:347 |
| var(--space-2) var(--space-1) | 1 | components/admin/CourseCatalogCourseDrawer.vue:377 |
| var(--space-3) 0 | 1 | components/admin/CourseCatalogCourseDrawer.vue:397 |
| var(--space-2) var(--space-3) var(--space-3) | 1 | components/catalog/CourseCard.vue:102 |
| var(--space-3) var(--space-4) var(--space-4) | 1 | components/catalog/CourseCard.vue:182 |
| var(--space-3) var(--space-4) | 1 | components/credential/CredentialSwitcher.vue:151 |
| 1px 5px | 1 | components/credential/CredentialSwitcher.vue:186 |
| var(--space-4) var(--space-5) | 1 | components/dashboard/QuickCard.vue:98 |
| var(--space-2) var(--space-5) var(--space-3) | 1 | components/dashboard/QuickCard.vue:129 |
| 2px 8px | 1 | components/dashboard/QuickCard.vue:180 |
| var(--space-8) var(--space-4) | 1 | components/dashboard/QuickCard.vue:204 |
| var(--space-3) var(--space-3) var(--space-1) | 1 | components/layout/AppSidebar.vue:230 |
| var(--space-2) var(--space-3) var(--space-1) calc(var(--space-3) + 12px) | 1 | components/layout/AppSidebar.vue:298 |
| 0 var(--space-3) | 1 | components/layout/AppSidebar.vue:335 |
| 0 var(--space-4) | 1 | components/layout/AppTopBar.vue:178 |
| var(--space-1) var(--space-4) var(--space-2) | 1 | components/layout/AppTopBar.vue:288 |
| 1px var(--space-2) | 1 | components/layout/AppTopBar.vue:303 |
| var(--space-1) var(--space-4) | 1 | components/layout/AppTopBar.vue:327 |
| 32px 0 | 1 | components/layout/NotificationPanelBody.vue:158 |
| 10px 4px | 1 | components/layout/NotificationPanelBody.vue:172 |
| 0 4px | 1 | components/layout/ProfileEditDialog.vue:134 |
| 0 20px | 1 | components/student/DocumentViewer.vue:281 |
| 6px 8px | 1 | components/student/ImageViewer.vue:323 |
| 2px | 1 | components/student/PptViewer.vue:409 |
| 6px 0 | 1 | components/student/PptViewer.vue:490 |
| 0 8px | 1 | components/student/PptViewer.vue:494 |
| 4px 8px | 1 | components/student/VideoPlayer.vue:266 |
| 18px 44px 14px 20px | 1 | components/ui/UiDialog.vue:162 |
| 14px 20px 18px | 1 | components/ui/UiDialog.vue:174 |
| var(--sp-10, 40px) 0 var(--sp-5, 20px) | 1 | components/valuation/PageHeader.vue:31 |
| var(--sp-6, 24px) 0 var(--sp-4, 16px) | 1 | components/valuation/PageHeader.vue:89 |
| 24px | 1 | pages/admin/ContentGenerate.vue:305 |
| 8px 10px | 1 | pages/admin/ContentGenerate.vue:453 |
| var(--space-6) var(--space-8) | 1 | pages/admin/Dashboard.vue:306 |
| var(--space-5) var(--space-6) | 1 | pages/admin/Dashboard.vue:409 |
| 32px 40px | 1 | pages/admin/FeaturedContentEdit.vue:371 |
| 20px 16px | 1 | pages/admin/FeaturedContentEdit.vue:436 |
| 8px 20px 8px 60px | 1 | pages/admin/ForumManage.vue:674 |
| 1px 6px | 1 | pages/admin/ForumManage.vue:739 |
| var(--sp-4) var(--sp-5) | 1 | pages/admin/ValuationConfigManage.vue:902 |
| 0 var(--sp-2) | 1 | pages/admin/ValuationConfigManage.vue:907 |
| var(--sp-3) var(--sp-2) var(--sp-5) | 1 | pages/admin/ValuationConfigManage.vue:910 |
| var(--sp-3) var(--sp-2) | 1 | pages/admin/ValuationConfigManage.vue:921 |
| 14px 16px | 1 | pages/ai-assistant/FeatureChatPage.vue:469 |
| 0 10px | 1 | pages/ai-assistant/FeatureChatPage.vue:510 |
| 6px 14px | 1 | pages/auth/Login.vue:374 |
| 32px | 1 | pages/onboarding/CredentialOnboarding.vue:141 |
| 14px | 1 | pages/onboarding/CredentialOnboarding.vue:180 |
| 2px 6px | 1 | pages/onboarding/CredentialOnboarding.vue:215 |
| 12px 16px | 1 | pages/student/ChapterView.vue:482 |
| 0 16px 14px | 1 | pages/student/HelpCenter.vue:163 |

## border（8 种取值 / 32 处）

| 取值 | 次数 | 落点（文件:行） |
| --- | --- | --- |
| 1px solid var(--color-border-light) | 13 | components/catalog/CourseCard.vue:72<br>components/catalog/FacetCard.vue:26<br>components/dashboard/QuickCard.vue:70<br>components/layout/NotificationPanel.vue:89<br>pages/admin/CourseCatalog.vue:575<br>pages/admin/RoleManage.vue:232<br>pages/ai-assistant/FeatureChatPage.vue:438<br>pages/ai-assistant/FeatureChatPage.vue:470<br>pages/ai-assistant/FeatureChatPage.vue:511<br>pages/ai-assistant/FeatureChatPage.vue:540<br>pages/ai-assistant/FeatureChatPage.vue:589<br>pages/onboarding/CredentialOnboarding.vue:139<br>pages/onboarding/CredentialOnboarding.vue:178 |
| none | 10 | components/ai-assistant/ChatPageShell.vue:818<br>components/catalog/FacetItem.vue:35<br>components/layout/AdminTabBar.vue:185<br>components/layout/AppSidebar.vue:337<br>components/layout/AppTopBar.vue:228<br>components/layout/AppTopBar.vue:266<br>components/layout/NotificationPanel.vue:63<br>components/student/DocumentViewer.vue:241<br>components/ui/UiInput.vue:104<br>pages/ai-assistant/FeatureChatPage.vue:599 |
| 1px solid var(--color-border) | 4 | components/auth/AuthPageShell.vue:192<br>components/practice/AnswerResultCard.vue:56<br>components/student/AnsweringSessionShell.vue:157<br>components/student/QuestionOptionPicker.vue:49 |
| 1px solid transparent | 1 | components/layout/AdminTabBar.vue:151 |
| 2px solid transparent | 1 | components/student/PptViewer.vue:401 |
| none !important | 1 | components/tutor/MarkdownEditor.vue:262 |
| 1px solid color-mix(in srgb, var(--color-violet-500) 18%, var(--color-border)) | 1 | pages/admin/Dashboard.vue:304 |
| 1px dashed var(--color-border-dark) | 1 | pages/admin/FeaturedContentEdit.vue:379 |
