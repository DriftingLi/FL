-- 000015: course 热门/精品标记（双 bool，可叠加）
ALTER TABLE course ADD COLUMN IF NOT EXISTS is_hot BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE course ADD COLUMN IF NOT EXISTS is_featured BOOLEAN NOT NULL DEFAULT FALSE;

CREATE INDEX IF NOT EXISTS idx_course_is_hot ON course (is_hot) WHERE is_hot = TRUE;
CREATE INDEX IF NOT EXISTS idx_course_is_featured ON course (is_featured) WHERE is_featured = TRUE;

COMMENT ON COLUMN course.is_hot IS '是否热门（运营精选）';
COMMENT ON COLUMN course.is_featured IS '是否精品（运营精选）';
