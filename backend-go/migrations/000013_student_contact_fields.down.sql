-- 000013_student_contact_fields 回滚：移除 phone/email/company 列及索引。
DROP INDEX IF EXISTS idx_student_phone;
ALTER TABLE student DROP COLUMN IF EXISTS phone;
ALTER TABLE student DROP COLUMN IF EXISTS email;
ALTER TABLE student DROP COLUMN IF EXISTS company;
