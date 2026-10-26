# 1957 附录 · scoped 取值逐条清单

> 由 scripts/inventory-ui-forms.mjs --md 生成，**不要手改**。
> 口径：<style scoped>（含 @media 内）的声明；括号里是出现次数。

## border-radius（14 种取值 / 68 处）

| 取值 | 次数 | 落点（文件:行） |
| --- | --- | --- |
| 8px | 16 | components/auth/AuthPageShell.vue:191<br>components/practice/AnswerResultCard.vue:57<br>components/student/AnsweringSessionShell.vue:145<br>components/student/AnsweringSessionShell.vue:151<br>components/student/DocumentViewer.vue:177<br>components/student/ImageViewer.vue:264<br>components/student/PptViewer.vue:274<br>components/student/QuestionOptionPicker.vue:49<br>components/student/VideoPlayer.vue:210<br>pages/admin/ContentGenerate.vue:427<br>pages/admin/FeaturedContentEdit.vue:380<br>pages/admin/FeaturedContentList.vue:247<br>pages/ai-assistant/FeatureChatPage.vue:512<br>pages/ai-assistant/FeatureChatPage.vue:541<br>pages/ai-assistant/FeatureChatPage.vue:588<br>pages/ai-assistant/FeatureChatPage.vue:616 |
| var(--radius-md) | 10 | components/catalog/FacetItem.vue:36<br>components/credential/CredentialSwitcher.vue:208<br>components/dashboard/QuickCard.vue:142<br>components/layout/AdminTabBar.vue:152<br>components/layout/AppSidebar.vue:285<br>components/layout/AppSidebar.vue:338<br>components/layout/AppSidebarItem.vue:96<br>components/layout/AppTopBar.vue:229<br>components/layout/AppTopBar.vue:267<br>pages/onboarding/CredentialOnboarding.vue:179 |
| var(--radius-lg) | 9 | components/catalog/FacetCard.vue:27<br>components/layout/NotificationPanel.vue:94<br>pages/admin/CourseCatalog.vue:576<br>pages/admin/Dashboard.vue:331<br>pages/admin/Dashboard.vue:362<br>pages/admin/Dashboard.vue:450<br>pages/admin/Dashboard.vue:466<br>pages/admin/RoleManage.vue:233<br>pages/onboarding/CredentialOnboarding.vue:140 |
| var(--radius-full) | 6 | components/credential/CredentialSwitcher.vue:187<br>components/dashboard/QuickCard.vue:181<br>components/layout/AdminTabBar.vue:186<br>components/layout/AppTopBar.vue:193<br>components/layout/AppTopBar.vue:304<br>pages/onboarding/CredentialOnboarding.vue:215 |
| 6px | 6 | components/student/ForumImageGallery.vue:38<br>components/student/QuestionOptionPicker.vue:56<br>pages/admin/AuditLogs.vue:142<br>pages/admin/ContentGenerate.vue:385<br>pages/admin/ForumManage.vue:728<br>pages/tutor/QuestionTags.vue:347 |
| 12px | 5 | components/auth/AuthPageShell.vue:204<br>components/auth/AuthPageShell.vue:210<br>pages/admin/ContentGenerate.vue:303<br>pages/admin/FeaturedContentEdit.vue:370<br>pages/ai-assistant/FeatureChatPage.vue:471 |
| var(--radius-xl) | 5 | components/catalog/CourseCard.vue:68<br>components/dashboard/QuickCard.vue:69<br>pages/admin/Dashboard.vue:305<br>pages/admin/Dashboard.vue:390<br>pages/admin/Dashboard.vue:424 |
| 50% | 3 | components/layout/NotificationPanelBody.vue:187<br>components/student/QuestionOptionPicker.vue:54<br>pages/ai-assistant/FeatureChatPage.vue:598 |
| 4px | 3 | components/student/AnsweringSessionShell.vue:156<br>components/student/PptViewer.vue:337<br>components/student/PptViewer.vue:402 |
| 0 var(--radius-full) var(--radius-full) 0 | 1 | components/layout/AppSidebarItem.vue:126 |
| var(--radius-md, 8px) | 1 | components/layout/NotificationPanel.vue:68 |
| 2px | 1 | components/student/PptViewer.vue:426 |
| 0 !important | 1 | components/tutor/MarkdownEditor.vue:263 |
| 999px | 1 | pages/ai-assistant/FeatureChatPage.vue:437 |

## box-shadow（13 种取值 / 21 处）

| 取值 | 次数 | 落点（文件:行） |
| --- | --- | --- |
| var(--shadow-xs) | 5 | components/catalog/CourseCard.vue:71<br>components/dashboard/QuickCard.vue:71<br>pages/admin/Dashboard.vue:332<br>pages/admin/Dashboard.vue:392<br>pages/admin/Dashboard.vue:426 |
| var(--shadow-sm) | 3 | components/catalog/CourseCard.vue:93<br>components/dashboard/QuickCard.vue:77<br>components/dashboard/QuickCard.vue:83 |
| var(--shadow-md) | 2 | components/dashboard/QuickCard.vue:90<br>pages/admin/Dashboard.vue:340 |
| 0 2px 12px rgba(0, 0, 0, 0.06) | 2 | pages/admin/ContentGenerate.vue:305<br>pages/admin/FeaturedContentEdit.vue:372 |
| none !important | 1 | components/ai-assistant/ChatPageShell.vue:824 |
| 0 0 0 1px var(--color-border) inset | 1 | components/auth/AuthPageShell.vue:212 |
| 0 0 0 1px var(--color-border-dark) inset | 1 | components/auth/AuthPageShell.vue:218 |
| 0 0 0 2px color-mix(in srgb, var(--color-primary-500) 20%, transparent) inset | 1 | components/auth/AuthPageShell.vue:223 |
| var(--shadow-lg) | 1 | components/catalog/CourseCard.vue:80 |
| 0 2px 4px rgba(0,0,0,0.1) | 1 | components/student/AnsweringSessionShell.vue:145 |
| 0 4px 20px rgba(0, 0, 0, 0.4) | 1 | components/student/PptViewer.vue:338 |
| none | 1 | components/ui/UiInput.vue:105 |
| 0 0 0 2px rgba(13, 148, 136, 0.15) | 1 | pages/onboarding/CredentialOnboarding.vue:192 |

## padding（75 种取值 / 135 处）

| 取值 | 次数 | 落点（文件:行） |
| --- | --- | --- |
| 20px | 13 | components/ui/UiDialog.vue:168<br>pages/admin/AISettings.vue:401<br>pages/admin/AuditLogs.vue:125<br>pages/admin/ContentGenerate.vue:283<br>pages/admin/Credentials.vue:227<br>pages/admin/FeaturedContentEdit.vue:348<br>pages/admin/FeaturedContentList.vue:219<br>pages/admin/HrwaiUserManage.vue:310<br>pages/admin/Statistics.vue:278<br>pages/admin/Statistics.vue:300<br>pages/admin/TutorManage.vue:264<br>pages/student/RealExamPractice.vue:195<br>pages/tutor/QuestionTags.vue:312 |
| 0 | 9 | components/ai-assistant/ChatPageShell.vue:823<br>components/catalog/FacetCard.vue:35<br>components/layout/AdminTabBar.vue:184<br>components/layout/AppSidebar.vue:362<br>components/student/SpreadsheetViewer.vue:86<br>components/ui/UiInput.vue:106<br>pages/admin/QuestionReview.vue:315<br>pages/admin/Statistics.vue:296<br>pages/ai-assistant/FeatureChatPage.vue:606 |
| 12px | 7 | pages/admin/AISettings.vue:474<br>pages/admin/AuditLogs.vue:140<br>pages/admin/ContentGenerate.vue:432<br>pages/admin/HrwaiUserManage.vue:329<br>pages/admin/Statistics.vue:357<br>pages/admin/TutorManage.vue:283<br>pages/tutor/QuestionTags.vue:395 |
| 16px | 6 | components/practice/AnswerResultCard.vue:58<br>components/student/PptViewer.vue:321<br>pages/admin/ContentGenerate.vue:425<br>pages/admin/ContentGenerate.vue:436<br>pages/admin/ContributionManage.vue:300<br>pages/admin/RecruiterManage.vue:348 |
| var(--space-3) var(--space-2) | 5 | components/catalog/FacetCard.vue:28<br>components/credential/CredentialSwitcher.vue:155<br>components/dashboard/QuickCard.vue:141<br>components/layout/AppSidebarItem.vue:131<br>pages/admin/CourseCatalog.vue:577 |
| var(--space-4) var(--space-5) | 4 | components/dashboard/QuickCard.vue:98<br>pages/admin/Dashboard.vue:396<br>pages/admin/Dashboard.vue:430<br>pages/admin/Dashboard.vue:435 |
| var(--space-4) | 4 | components/layout/NotificationPanel.vue:96<br>layouts/SidebarLayout.vue:249<br>pages/admin/Dashboard.vue:449<br>pages/admin/RoleManage.vue:231 |
| 8px 12px | 4 | components/student/DocumentViewer.vue:186<br>components/student/ImageViewer.vue:272<br>components/student/QuestionOptionPicker.vue:56<br>pages/ai-assistant/FeatureChatPage.vue:539 |
| 0 var(--space-2) | 3 | components/layout/AppTopBar.vue:187<br>components/layout/AppTopBar.vue:265<br>components/layout/AppTopBar.vue:360 |
| 4px 14px | 2 | components/auth/AuthPageShell.vue:211<br>pages/ai-assistant/FeatureChatPage.vue:436 |
| var(--space-1) var(--space-3) | 2 | components/catalog/FacetCard.vue:41<br>components/layout/AdminTabBar.vue:150 |
| var(--space-2) var(--space-3) | 2 | components/catalog/FacetItem.vue:33<br>components/layout/AppSidebarItem.vue:95 |
| 0 var(--space-6) | 2 | components/layout/AdminTabBar.vue:133<br>components/layout/AppTopBar.vue:246 |
| var(--space-2) var(--space-2) | 2 | components/layout/AppSidebar.vue:204<br>components/layout/AppSidebar.vue:325 |
| var(--space-2) | 2 | components/layout/AppSidebar.vue:275<br>pages/admin/CourseCatalog.vue:584 |
| 8px 0 | 2 | components/practice/CommentCard.vue:97<br>components/student/PptViewer.vue:386 |
| 10px 15px | 2 | components/student/AnsweringSessionShell.vue:145<br>components/student/QuestionOptionPicker.vue:49 |
| 8px 16px | 2 | components/student/PptViewer.vue:287<br>components/student/PptViewer.vue:355 |
| 0 16px | 2 | components/student/PptViewer.vue:394<br>pages/student/HelpCenter.vue:150 |
| 8px | 2 | components/student/PptViewer.vue:473<br>components/student/PptViewer.vue:477 |
| 40px 20px | 2 | components/student/SpreadsheetViewer.vue:90<br>components/student/VideoPlayer.vue:271 |
| var(--space-6) | 2 | layouts/SidebarLayout.vue:173<br>pages/onboarding/CredentialOnboarding.vue:133 |
| 10px 12px | 2 | pages/admin/ContentGenerate.vue:384<br>pages/tutor/QuestionTags.vue:344 |
| var(--space-2) var(--space-1) | 1 | components/admin/CourseCatalogCourseDrawer.vue:377 |
| var(--space-3) 0 | 1 | components/admin/CourseCatalogCourseDrawer.vue:397 |
| var(--space-2) var(--space-3) var(--space-3) | 1 | components/catalog/CourseCard.vue:102 |
| var(--space-3) var(--space-4) var(--space-4) | 1 | components/catalog/CourseCard.vue:182 |
| var(--space-3) var(--space-4) | 1 | components/credential/CredentialSwitcher.vue:151 |
| 1px 5px | 1 | components/credential/CredentialSwitcher.vue:186 |
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
| 0 4px | 1 | components/layout/ProfileEditDialog.vue:133 |
| 0 20px | 1 | components/student/DocumentViewer.vue:281 |
| 6px 8px | 1 | components/student/ImageViewer.vue:322 |
| 2px | 1 | components/student/PptViewer.vue:409 |
| 6px 0 | 1 | components/student/PptViewer.vue:490 |
| 0 8px | 1 | components/student/PptViewer.vue:494 |
| 4px 8px | 1 | components/student/VideoPlayer.vue:266 |
| 18px 44px 14px 20px | 1 | components/ui/UiDialog.vue:162 |
| 14px 20px 18px | 1 | components/ui/UiDialog.vue:174 |
| var(--sp-10, 40px) 0 var(--sp-5, 20px) | 1 | components/valuation/PageHeader.vue:31 |
| var(--sp-6, 24px) 0 var(--sp-4, 16px) | 1 | components/valuation/PageHeader.vue:89 |
| 24px | 1 | pages/admin/ContentGenerate.vue:304 |
| 8px 10px | 1 | pages/admin/ContentGenerate.vue:452 |
| var(--space-6) var(--space-8) | 1 | pages/admin/Dashboard.vue:306 |
| var(--space-5) | 1 | pages/admin/Dashboard.vue:356 |
| var(--space-3) var(--space-5) | 1 | pages/admin/Dashboard.vue:401 |
| var(--space-5) var(--space-6) | 1 | pages/admin/Dashboard.vue:499 |
| 32px 40px | 1 | pages/admin/FeaturedContentEdit.vue:371 |
| 20px 16px | 1 | pages/admin/FeaturedContentEdit.vue:436 |
| 8px 20px 8px 60px | 1 | pages/admin/ForumManage.vue:664 |
| 1px 6px | 1 | pages/admin/ForumManage.vue:729 |
| 10px 16px | 1 | pages/admin/Statistics.vue:329 |
| var(--sp-4) var(--sp-5) | 1 | pages/admin/ValuationConfigManage.vue:904 |
| 0 var(--sp-2) | 1 | pages/admin/ValuationConfigManage.vue:909 |
| var(--sp-3) var(--sp-2) var(--sp-5) | 1 | pages/admin/ValuationConfigManage.vue:912 |
| var(--sp-3) var(--sp-2) | 1 | pages/admin/ValuationConfigManage.vue:923 |
| 14px 16px | 1 | pages/ai-assistant/FeatureChatPage.vue:469 |
| 0 10px | 1 | pages/ai-assistant/FeatureChatPage.vue:510 |
| 6px 14px | 1 | pages/auth/Login.vue:374 |
| 32px | 1 | pages/onboarding/CredentialOnboarding.vue:141 |
| 14px | 1 | pages/onboarding/CredentialOnboarding.vue:180 |
| 2px 6px | 1 | pages/onboarding/CredentialOnboarding.vue:214 |
| 12px 16px | 1 | pages/student/ChapterView.vue:482 |
| 0 16px 14px | 1 | pages/student/HelpCenter.vue:163 |

## border（8 种取值 / 35 处）

| 取值 | 次数 | 落点（文件:行） |
| --- | --- | --- |
| 1px solid var(--color-border-light) | 16 | components/catalog/CourseCard.vue:72<br>components/catalog/FacetCard.vue:26<br>components/dashboard/QuickCard.vue:70<br>components/layout/NotificationPanel.vue:93<br>pages/admin/CourseCatalog.vue:575<br>pages/admin/Dashboard.vue:333<br>pages/admin/Dashboard.vue:391<br>pages/admin/Dashboard.vue:425<br>pages/admin/RoleManage.vue:232<br>pages/ai-assistant/FeatureChatPage.vue:438<br>pages/ai-assistant/FeatureChatPage.vue:470<br>pages/ai-assistant/FeatureChatPage.vue:511<br>pages/ai-assistant/FeatureChatPage.vue:540<br>pages/ai-assistant/FeatureChatPage.vue:589<br>pages/onboarding/CredentialOnboarding.vue:139<br>pages/onboarding/CredentialOnboarding.vue:178 |
| none | 10 | components/ai-assistant/ChatPageShell.vue:821<br>components/catalog/FacetItem.vue:35<br>components/layout/AdminTabBar.vue:185<br>components/layout/AppSidebar.vue:337<br>components/layout/AppTopBar.vue:228<br>components/layout/AppTopBar.vue:266<br>components/layout/NotificationPanel.vue:67<br>components/student/DocumentViewer.vue:241<br>components/ui/UiInput.vue:104<br>pages/ai-assistant/FeatureChatPage.vue:599 |
| 1px solid var(--color-border) | 4 | components/auth/AuthPageShell.vue:192<br>components/practice/AnswerResultCard.vue:56<br>components/student/AnsweringSessionShell.vue:156<br>components/student/QuestionOptionPicker.vue:49 |
| 1px solid transparent | 1 | components/layout/AdminTabBar.vue:151 |
| 2px solid transparent | 1 | components/student/PptViewer.vue:401 |
| none !important | 1 | components/tutor/MarkdownEditor.vue:262 |
| 1px solid color-mix(in srgb, var(--color-violet-500) 18%, var(--color-border)) | 1 | pages/admin/Dashboard.vue:304 |
| 1px dashed var(--color-border-dark) | 1 | pages/admin/FeaturedContentEdit.vue:379 |
