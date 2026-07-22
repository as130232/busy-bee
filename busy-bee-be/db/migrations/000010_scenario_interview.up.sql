-- 新增 interview（面試）情境：放寬 scenario CHECK 約束。
ALTER TABLE meetings
    DROP CONSTRAINT meetings_scenario_check;

ALTER TABLE meetings
    ADD CONSTRAINT meetings_scenario_check CHECK (scenario IN ('meeting', 'casual', 'interview'));
