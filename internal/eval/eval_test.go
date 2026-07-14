package eval

import "testing"

func TestAUC(t *testing.T) {
	cases := []struct {
		name   string
		scores []float64
		labels []int
		want   float64
	}{
		{"perfect", []float64{3, 2, 1, 0}, []int{1, 1, 0, 0}, 1.0},
		{"inverted", []float64{3, 2, 1, 0}, []int{0, 0, 1, 1}, 0.0},
		{"tie counts half", []float64{1, 1}, []int{1, 0}, 0.5},
		{"no positives", []float64{1, 2}, []int{0, 0}, 0.5},
		{"one each, pos higher", []float64{5, 1}, []int{1, 0}, 1.0},
	}
	for _, c := range cases {
		if got := auc(c.scores, c.labels); got != c.want {
			t.Errorf("%s: auc = %.3f, want %.3f", c.name, got, c.want)
		}
	}
}

func TestPrecisionAtK(t *testing.T) {
	// scores 4,3,2,1 with labels: only the 2nd and 4th ranked are positive.
	scores := []float64{4, 3, 2, 1}
	labels := []int{0, 1, 0, 1}
	order := rankOrder(scores) // [0,1,2,3]
	if got := precisionAtK(order, labels, 2); got != 0.5 {
		t.Errorf("P@2 = %.3f, want 0.5", got)
	}
	if got := precisionAtK(order, labels, 4); got != 0.5 {
		t.Errorf("P@4 = %.3f, want 0.5", got)
	}
	if got := precisionAtK(order, labels, 10); got != 0.5 { // k clamps to n
		t.Errorf("P@10 = %.3f, want 0.5 (clamped)", got)
	}
}

func TestIFA(t *testing.T) {
	scores := []float64{4, 3, 2, 1}
	order := rankOrder(scores)
	if got := ifa(order, []int{0, 0, 1, 0}); got != 3 {
		t.Errorf("ifa = %.1f, want 3", got)
	}
	if got := ifa(order, []int{1, 0, 0, 0}); got != 1 {
		t.Errorf("ifa = %.1f, want 1", got)
	}
	if got := ifa(order, []int{0, 0, 0, 0}); got != 5 { // none → N+1
		t.Errorf("ifa (no positives) = %.1f, want 5", got)
	}
}

func TestRankOrderDescending(t *testing.T) {
	order := rankOrder([]float64{1, 5, 3})
	want := []int{1, 2, 0} // indices of 5, 3, 1
	for i := range want {
		if order[i] != want[i] {
			t.Fatalf("rankOrder = %v, want %v", order, want)
		}
	}
}

func TestFilesOverlap(t *testing.T) {
	if !filesOverlap([]string{"a.go", "b.go"}, []string{"x.go", "b.go"}) {
		t.Error("expected overlap on b.go")
	}
	if filesOverlap([]string{"a.go"}, []string{"b.go"}) {
		t.Error("unexpected overlap")
	}
}

func TestRevertRegexParsesFullBody(t *testing.T) {
	body := "Some subject\n\nThis reverts commit 2c2d5e09d69412b06136b60446c97738594298ff.\n"
	m := revertMsgRe.FindStringSubmatch(body)
	if m == nil || m[1] != "2c2d5e09d69412b06136b60446c97738594298ff" {
		t.Fatalf("revert regex failed: %v", m)
	}
}
