-- #517 资料投稿域回滚：删除全部投稿相关表（含依赖文件/下载/举报）。
DROP TABLE IF EXISTS contribution_report;
DROP TABLE IF EXISTS contribution_download;
DROP TABLE IF EXISTS user_contribution_file;
DROP TABLE IF EXISTS user_contribution;
