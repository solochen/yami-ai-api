package handler

import "testing"

func TestMergeContinuationStitchesCleanly(t *testing.T) {
	cases := []struct {
		name string
		prev string
		next string
		want string
	}{
		{"number list continues", "1,2,3,4,5", "6,7,8", "1,2,3,4,5,6,7,8"},
		{"missing comma restored", "1,2,3", "4,5", "1,2,3,4,5"},
		{"doubled comma removed", "1,2,3,4,5,6,7,8,", ",9,10", "1,2,3,4,5,6,7,8,9,10"},
		{"doubled comma with spaces", "1, 2, 3, 4, 5, 6, 7, 8, ", ", 9, 10", "1, 2, 3, 4, 5, 6, 7, 8, 9, 10"},
		{"repeated block restarts", "1,2,3,4,5,6,7,8,9,10,11,12,13,14,15", "10,11,12,13,14,15,16", "1,2,3,4,5,6,7,8,9,10,11,12,13,14,15,16"},
		{"exact tail repeated", "从1数到100，", "从1数到100，例如", "从1数到100，例如"},
		{"prose keeps one space", "The model was trained on", "on large datasets.", "The model was trained on large datasets."},
		{"cjk overlap", "今天我去了公司", "公司开会", "今天我去了公司开会"},
		{"empty continuation", "abc", "", "abc"},
		{"no overlap prose", "第一段结束。", "第二段开始。", "第一段结束。第二段开始。"},
	}
	for _, tc := range cases {
		if got := mergeContinuation(tc.prev, tc.next); got != tc.want {
			t.Fatalf("%s: mergeContinuation(%q, %q) = %q, want %q", tc.name, tc.prev, tc.next, got, tc.want)
		}
	}
}
