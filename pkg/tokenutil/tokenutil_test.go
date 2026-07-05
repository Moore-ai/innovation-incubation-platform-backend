package tokenutil

import "testing"

func TestEstimate_Simple(t *testing.T) {
	SetEstimationMode("simple")

	tests := []struct {
		input string
		want  int
		msg   string
	}{
		{"你好", 2, "2 CJK chars"},
		{"你好!", 2, "2 CJK + '你好!' has CJK, not counted"},
		{"Hello World", 2, "2 non-CJK words"},
		{"", 0, "empty"},
		{"   ", 0, "spaces"},
		{"你好世界", 4, "4 CJK chars"},
		{"a b c", 3, "3 words"},
		{"数字化转型 subsidy", 6, "5 CJK + 1 non-CJK word"},
		{`{"text": object}`, 2, "2 non-CJK words"},
	}

	for _, tt := range tests {
		t.Run(tt.msg, func(t *testing.T) {
			got := Estimate(tt.input)
			if got != tt.want {
				t.Errorf("Estimate(%q) = %d, want %d", tt.input, got, tt.want)
			}
		})
	}
}

func TestEstimate_Better(t *testing.T) {
	SetEstimationMode("better")

	tests := []struct {
		input string
		want  int
		msg   string
	}{
		{"你好", 3, "2*1.5=3"},
		{"你好!", 3, "int(3+1/3.5)=3"},
		{"Hello World", 3, "int(11/3.5)=3"},
		{"", 0, "empty"},
		{"你好世界", 6, "4*1.5=6"},
		{"数字化转型 subsidy", 10, "int(5*1.6+16/3.5)=10"},
		{"a", 0, "int(1/3.5)=0"},
	}

	for _, tt := range tests {
		t.Run(tt.msg, func(t *testing.T) {
			got := Estimate(tt.input)
			if got != tt.want {
				t.Errorf("Estimate(%q) = %d, want %d", tt.input, got, tt.want)
			}
		})
	}
}

func TestSetEstimationMode_Defaults(t *testing.T) {
	SetEstimationMode("better")
	if estimationMode != "better" {
		t.Error("default should be better")
	}

	SetEstimationMode("unknown")
	if estimationMode == "unknown" {
		t.Error("unknown should fallback to simple")
	}
}

func TestIsCJK(t *testing.T) {
	cases := []struct {
		r    rune
		want bool
	}{
		{'你', true},
		{'好', true},
		{'a', false},
		{'1', false},
		{'+', false},
		{'\n', false},
		{' ', false},
		{'㐀', true},
		{'一', true},
	}
	for _, c := range cases {
		got := IsCJK(c.r)
		if got != c.want {
			t.Errorf("IsCJK(%q) = %v, want %v", c.r, got, c.want)
		}
	}
}
