package progress

import "testing"

func TestOutputMode(t *testing.T) {
	t.Cleanup(func() { SetOutputMode(false, false) })

	SetOutputMode(true, false)
	if !IsQuiet() || IsJSON() || ShowHumanOutput() {
		t.Fatalf("quiet mode: quiet=%v json=%v human=%v", IsQuiet(), IsJSON(), ShowHumanOutput())
	}

	SetOutputMode(false, true)
	if IsQuiet() || !IsJSON() || ShowHumanOutput() {
		t.Fatalf("json mode: quiet=%v json=%v human=%v", IsQuiet(), IsJSON(), ShowHumanOutput())
	}
}
