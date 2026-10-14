package service

import (
	"strings"
	"testing"
)

func TestSanitizeSubDir(t *testing.T) {
	cases := map[string]string{
		"":                          "",
		"   ":                       "",
		"/note":                     "note",
		"note":                      "note",
		"note/":                     "note",
		"/note/":                    "note",
		"note/u":                    "note/u",
		"note/../../etc":            "etc", // 多层塌缩，.. 被丢
		"../escape":                 "escape",
		"a/b/../c":                  "a/c",
		"//foo//bar//":              "foo/bar",
		strings.Repeat("x", 40):     strings.Repeat("x", 32), // 超长截断
	}
	for in, want := range cases {
		if got := sanitizeSubDir(in); got != want {
			t.Errorf("sanitizeSubDir(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestSanitizeSubDir_PreservesEmptyForDefault(t *testing.T) {
	// subDir 空 → 走 s.dir 默认目录
	if sanitizeSubDir("") != "" {
		t.Error("expected empty for empty input")
	}
}
