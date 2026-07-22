ALTER TABLE meetings DROP CONSTRAINT IF EXISTS meetings_scenario_check;
ALTER TABLE meetings DROP COLUMN IF EXISTS summary_sections;
ALTER TABLE meetings DROP COLUMN IF EXISTS scenario;
