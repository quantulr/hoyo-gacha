package main

import (
	"errors"
	"net/url"
	"strings"
	"testing"
)

func TestConvertGachaTime(t *testing.T) {
	if got := convertGachaTime("2024-01-01 10:00:00", 8, -5); got != "2023-12-31 21:00:00" {
		t.Errorf("东八区到美东 = %q", got)
	}
	if got := convertGachaTime("2024-01-01 10:00:00", 8, 8); got != "2024-01-01 10:00:00" {
		t.Errorf("同一时区 = %q", got)
	}
}

func TestSanitizeURLError(t *testing.T) {
	err := &url.Error{Op: "Get", URL: "https://example.com/?authkey=SECRET", Err: errors.New("connection reset")}
	got := sanitizeURLError(err)
	if got == nil || strings.Contains(got.Error(), "SECRET") {
		t.Fatalf("错误里仍带有链接：%v", got)
	}
	var urlErr *url.Error
	if errors.As(got, &urlErr) {
		t.Errorf("仍可取出 url.Error：%v", got)
	}
}
