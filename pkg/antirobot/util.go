package antirobot

import (
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// CollapseSpace 将连续空白（含换行、制表符）压缩为单个空格并 TrimSpace。
func CollapseSpace(s string) string {
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\r", " ")
	s = strings.ReplaceAll(s, "\t", " ")
	return strings.Join(strings.Fields(s), " ")
}

// doiRe DOI 模式：正则字符类不含空格，天然避免吃进后续文本；(?i) 覆盖大写 DOI。
var doiRe = regexp.MustCompile(`(?i)10\.\d{4,9}/[-._;()/:A-Z0-9]+`)

// ExtractDOI 从任意文本/URL 提取 DOI，未命中返回空串。
// 结尾标点（句号、逗号、右括号等）被剥离。
func ExtractDOI(s string) string {
	m := doiRe.FindString(s)
	return strings.TrimRight(m, ".,;)>]")
}

// StripXMLTags 移除字符串中的 XML/HTML 标签。
func StripXMLTags(s string) string {
	var sb strings.Builder
	inTag := false
	for _, r := range s {
		switch {
		case r == '<':
			inTag = true
		case r == '>':
			inTag = false
		case !inTag:
			sb.WriteRune(r)
		}
	}
	return sb.String()
}

// ParseRetryAfter 解析 Retry-After 响应头：支持秒数与 HTTP-Date 两种格式，
// 缺失或解析失败返回 0。
func ParseRetryAfter(v string) time.Duration {
	v = strings.TrimSpace(v)
	if v == "" {
		return 0
	}
	if secs, err := strconv.Atoi(v); err == nil && secs > 0 {
		return time.Duration(secs) * time.Second
	}
	if t, err := http.ParseTime(v); err == nil {
		if d := time.Until(t); d > 0 {
			return d
		}
	}
	return 0
}
