-- 回退 idea 情境（既有 idea 資料需先清除或改值，否則 CHECK 失敗）。
ALTER TABLE meetings
    DROP CONSTRAINT meetings_scenario_check;

ALTER TABLE meetings
    ADD CONSTRAINT meetings_scenario_check CHECK (scenario IN ('meeting', 'casual', 'interview'));
