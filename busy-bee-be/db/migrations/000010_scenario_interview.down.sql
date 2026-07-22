-- 回復為僅 meeting/casual；先把既有 interview 資料收斂回 meeting，避免違反約束。
UPDATE meetings SET scenario = 'meeting' WHERE scenario = 'interview';

ALTER TABLE meetings
    DROP CONSTRAINT meetings_scenario_check;

ALTER TABLE meetings
    ADD CONSTRAINT meetings_scenario_check CHECK (scenario IN ('meeting', 'casual'));
