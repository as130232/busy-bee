package meeting

import "testing"

func TestParseScenario(t *testing.T) {
	cases := []struct {
		in   string
		want Scenario
	}{
		{"meeting", ScenarioMeeting},
		{"casual", ScenarioCasual},
		{"interview", ScenarioInterview},
		{"", ScenarioMeeting},        // 空值回退預設
		{"unknown", ScenarioMeeting}, // 未知值回退預設（容忍舊資料/未來值）
		{"MEETING", ScenarioMeeting}, // 大小寫不符即無效，回退
	}
	for _, c := range cases {
		if got := ParseScenario(c.in); got != c.want {
			t.Errorf("ParseScenario(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestScenarioIsValid(t *testing.T) {
	if !ScenarioMeeting.IsValid() || !ScenarioCasual.IsValid() || !ScenarioInterview.IsValid() {
		t.Error("meeting/casual/interview should be valid")
	}
	if Scenario("nope").IsValid() {
		t.Error("unknown scenario should be invalid")
	}
}
