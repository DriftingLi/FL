-- 000013_student_contact_fields: 为学员表新增手机号/邮箱/单位字段，支撑注册字段重构。
-- 手机号必填且唯一；邮箱与单位选填。username 仍保留唯一约束，注册时由后端置为手机号。

-- 1. 新增三列（先允许 NULL 以便回填，再对 phone 设置 NOT NULL）
ALTER TABLE student ADD COLUMN phone   VARCHAR(20)  ;
ALTER TABLE student ADD COLUMN email   VARCHAR(100);
ALTER TABLE student ADD COLUMN company VARCHAR(100);

-- 2. 用已有 username 回填 phone，满足后续 NOT NULL 约束
UPDATE student SET phone = username WHERE phone IS NULL;

-- 3. phone 设置 NOT NULL 并加唯一索引
ALTER TABLE student ALTER COLUMN phone SET NOT NULL;
CREATE UNIQUE INDEX idx_student_phone ON student (phone);

-- 4. 列注释
COMMENT ON COLUMN student.phone   IS '手机号（注册时必填，同时作为自动生成的用户名）';
COMMENT ON COLUMN student.email   IS '邮箱（选填）';
COMMENT ON COLUMN student.company IS '单位/公司（选填）';
