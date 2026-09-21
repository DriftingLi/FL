-- #1197（ADR-0061 §2）：裁决窗口只属于 pending。
--
-- 背景：`expires_at` 曾被当成对全部状态通用的列——`EnsureApproved` 给 `approved` 行也写
-- now+14d，而门禁（`contact_authz.go`）从不读它 ⇒ 一列对 `approved` 永远说谎。
-- 领域口径见 CONTEXT.md 的「裁决窗口（decision window）」：它是 **pending 的属性**，
-- `approved` 是永久授权（出口只有学员撤回与学员注销）。
--
-- 为什么 DROP NOT NULL 而不是保留非空 + 哨兵值：非空约束强迫代码给「无期限」编一个日期，
-- 远期哨兵（9999-12-31）会把「裁决窗口」和「授权寿命」两个刚分开的概念重新糊成一个，
-- 且任何 `expires_at > now()` 的误读都会把永久授权算成"还剩很久"。
--
-- 为什么同时加 CHECK：本案的教训正是「应用层约定会漏」（14 天过期守护漏装了三周）。
-- "pending 必须有窗口" 是状态机的结构事实，钉在库层才不依赖某个调用点记得写。
-- `approved/rejected/expired/revoked` 可空——`expired` 保留其值，因为它记录了窗口何时关闭。
--
-- 不加 `NOT NULL DEFAULT`：DEFAULT 会在插入路径上重新制造"给 approved 写值"的隐式行为。
--
-- 存量数据无需回填：全部存量行满足 `expires_at = created_at + 14d`（生产只有两处写入点，
-- 且都写该式），故 pending 行照旧非空，approved 行的旧值变成"未被读取的历史值"——
-- 不删（它仍是签发时的真实快照，删掉才是销毁事实），只是不再有任何判据读它。

ALTER TABLE contact_requests
    ALTER COLUMN expires_at DROP NOT NULL;

ALTER TABLE contact_requests
    ADD CONSTRAINT chk_contact_requests_pending_window
    CHECK (status <> 'pending' OR expires_at IS NOT NULL);

COMMENT ON COLUMN contact_requests.expires_at IS
    '裁决窗口关闭时刻（ADR-0061 §2）：仅 pending 有值，= 签发时刻 + 当时约定的窗口长度（历史快照，改窗口不回溯）。approved/rejected/expired/revoked 可空或不再被读';
