package antirobot

import "testing"

func TestExtractDOI(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		// URL 路径中的 DOI
		{"https://doi.org/10.1038/nature12373", "10.1038/nature12373"},
		{"https://www.nature.com/articles/10.1038/nature12373", "10.1038/nature12373"},
		// 句尾标点剥离
		{"...as cited in 10.1038/nature12373.", "10.1038/nature12373"},
		{"see 10.1038/nature12373, and refs therein", "10.1038/nature12373"},
		// 大写与特殊字符；尾部 ) 按"引用包裹符"剥掉（真实 DOI 极少以 ) 结尾，
		// 换取 "as in (10.1038/xxx)" 场景的正确剥离）
		{"10.1234/UPPER-CASE(2024)", "10.1234/UPPER-CASE(2024"},
		{"(10.1038/nature12373)", "10.1038/nature12373"},
		// 无 DOI
		{"https://arxiv.org/abs/1706.03762", ""},
		{"no doi in this text", ""},
		{"", ""},
	}
	for _, c := range cases {
		if got := ExtractDOI(c.in); got != c.want {
			t.Errorf("ExtractDOI(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}
