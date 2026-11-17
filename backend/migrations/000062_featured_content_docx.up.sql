-- 内容精选支持「导入原始 .docx 文档」（2026-11）。
--
-- 为什么是**新增两列**而不是把 docx 转成 Markdown 再存进 content：
--   * 用户要的是「直接渲染 .docx」，转 Markdown 会丢排版（页眉页脚、多级列表缩进、
--     合并单元格、字体字号），恰好是本票要保住的那部分；
--   * 转出来的 Markdown 还多一个「转换器口径」要维护，且不可逆；
--   * 存原始文件 + 类型标记，读者端用 docx-preview 按 OOXML 渲染，作者看到的就是 Word 里的样子。
--
-- 两条不变量（write 侧由 service 维持，库层兜住第二条）：
--   1. `content_type = 'docx'` ⇒ `docx_url` 非空（没有文件就没有正文可渲染）；
--   2. `content_type` 只取 markdown / docx。
-- 第二条用 CHECK 钉住；第一条不加 CHECK —— 与 000043 的取舍同源：存量语义由写库侧兜，
-- 库层只钉「取值域」这种永不误伤历史数据的约束。
--
-- 存量行全部是 Markdown 正文 ⇒ 默认值 'markdown' 让本迁移之前写入的内容无需回填。

ALTER TABLE featured_content
    ADD COLUMN IF NOT EXISTS content_type VARCHAR(16) NOT NULL DEFAULT 'markdown';

ALTER TABLE featured_content
    ADD COLUMN IF NOT EXISTS docx_url TEXT NOT NULL DEFAULT '';

COMMENT ON COLUMN featured_content.content_type IS
    '正文形态（2026-11）：markdown = 编辑器正文走 Markdown 渲染；docx = 导入原始 Word，正文改看 docx_url';

COMMENT ON COLUMN featured_content.docx_url IS
    '原始 .docx 文件 URL（featured/docx/ 子目录）；content_type=docx 时非空，读者端用 docx-preview 直接渲染';

ALTER TABLE featured_content
    ADD CONSTRAINT chk_featured_content_content_type
    CHECK (content_type IN ('markdown', 'docx'));
