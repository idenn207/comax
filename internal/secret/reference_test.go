package secret

import (
	"testing"
)

func TestFindReferences(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want []reference
	}{
		{"empty string", "", nil},
		{"no references", "plain text", nil},
		{"single ref", "prefix ${{ shared.DB_HOST }} suffix", []reference{
			{envName: "shared", keyName: "DB_HOST", start: 7, end: 28},
		}},
		{"two refs in one value", "${{a.X}}-${{b.Y}}", []reference{
			{envName: "a", keyName: "X", start: 0, end: 8},
			{envName: "b", keyName: "Y", start: 9, end: 17},
		}},
		{"tolerates whitespace", "${{   shared.HOST   }}", []reference{
			{envName: "shared", keyName: "HOST", start: 0, end: 22},
		}},
		{"non-ref braces are literal", "name = {{ not a ref }}", nil},
		{"missing dot is literal", "${{ shared }}", nil},
		{"disallowed chars in name are literal", "${{ sh@red.X }}", nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := findReferences(tc.in)
			if len(got) != len(tc.want) {
				t.Fatalf("got %d refs, want %d (refs=%+v)", len(got), len(tc.want), got)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Errorf("ref[%d] = %+v; want %+v", i, got[i], tc.want[i])
				}
			}
		})
	}
}
