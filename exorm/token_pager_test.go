package exorm

import (
	"encoding/base64"
	"testing"
)

type testTokenPager struct {
	size  int64
	token string
}

func (p testTokenPager) GetPageSize() int64   { return p.size }
func (p testTokenPager) GetPageToken() string { return p.token }

var _ TokenPager = testTokenPager{}

func TestPageTokenRoundTrip(t *testing.T) {
	for _, cursor := range []int64{0, 1, 20, 1 << 40} {
		token := EncodePageToken(cursor)

		if got := DecodePageToken(token); got != cursor {
			t.Fatalf("DecodePageToken(EncodePageToken(%d)) = %d", cursor, got)
		}
	}
}

func TestDecodePageTokenInvalid(t *testing.T) {
	tests := []string{
		"!!!",
		base64.RawURLEncoding.EncodeToString([]byte("abc")),
	}

	for _, token := range tests {
		t.Run(token, func(t *testing.T) {
			if got := DecodePageToken(token); got != 0 {
				t.Fatalf("DecodePageToken(%q) = %d, want 0", token, got)
			}
		})
	}
}

func TestNextPageToken(t *testing.T) {
	if got := NextPageToken(100, 10, 10); got != EncodePageToken(100) {
		t.Errorf("NextPageToken(100, 10, 10) = %q, want token for cursor 100", got)
	}
	if got := NextPageToken(100, 9, 10); got != "" {
		t.Errorf("NextPageToken(100, 9, 10) = %q, want empty", got)
	}
	if got := NextPageToken(100, 0, 10); got != "" {
		t.Errorf("NextPageToken(100, 0, 10) = %q, want empty", got)
	}
	if got := NextPageToken(100, 10, 0); got != "" {
		t.Errorf("NextPageToken(100, 10, 0) = %q, want empty", got)
	}
}
