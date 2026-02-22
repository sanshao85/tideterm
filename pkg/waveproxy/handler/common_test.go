package handler

import (
	"net/http"
	"testing"
)

func TestCopyHeadersForUpstreamRequestFiltersAcceptEncoding(t *testing.T) {
	src := http.Header{}
	src.Set("Accept-Encoding", "gzip, deflate, br")
	src.Set("User-Agent", "claude-cli/2.0.76 (external, cli)")
	src.Set("Accept", "application/json")
	src.Set("Authorization", "Bearer should-not-forward")
	src.Set("X-API-Key", "should-not-forward")
	src.Set("Content-Length", "123")

	dst := http.Header{}
	copyHeadersForUpstreamRequest(dst, src)

	if dst.Get("Accept-Encoding") != "" {
		t.Fatalf("expected Accept-Encoding to be filtered, got %q", dst.Get("Accept-Encoding"))
	}
	if dst.Get("User-Agent") == "" {
		t.Fatalf("expected User-Agent to be copied")
	}
	if dst.Get("Accept") == "" {
		t.Fatalf("expected Accept to be copied")
	}
	if dst.Get("Authorization") != "" {
		t.Fatalf("expected Authorization to be filtered, got %q", dst.Get("Authorization"))
	}
	if dst.Get("X-API-Key") != "" {
		t.Fatalf("expected X-API-Key to be filtered, got %q", dst.Get("X-API-Key"))
	}
	if dst.Get("Content-Length") != "" {
		t.Fatalf("expected Content-Length to be filtered, got %q", dst.Get("Content-Length"))
	}
}

