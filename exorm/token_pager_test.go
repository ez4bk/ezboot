package exorm

import (
	"errors"
	"strings"
	"testing"

	_ "github.com/go-sql-driver/mysql"
	"xorm.io/xorm"
)

type testTokenPager struct {
	size  int64
	token string
}

func (p testTokenPager) GetPageSize() int64   { return p.size }
func (p testTokenPager) GetPageToken() string { return p.token }

var _ TokenPager = testTokenPager{}

func TestDefaultTokenRoundTrip(t *testing.T) {
	tests := map[string]any{
		"int64_zero":      int64(0),
		"int64_large":     int64(1 << 40),
		"uint64_high_bit": uint64(1 << 63),
		"uint64_max":      ^uint64(0),
		"string_uuid":     "550e8400-e29b-41d4-a716-446655440000",
		"string_empty":    "",
	}

	for name, pk := range tests {
		t.Run(name, func(t *testing.T) {
			raw := NewDefaultToken("id", pk)

			decoded, err := DefaultTokenDecoder("id")(raw.Encode())
			if err != nil {
				t.Fatalf("decode token for pk %v: %v", pk, err)
			}

			token, ok := decoded.(DefaultToken)
			if !ok {
				t.Fatalf("decoded token type = %T, want DefaultToken", decoded)
			}
			if token.pk != pk {
				t.Fatalf("decoded pk = %v(%T), want %v(%T)", token.pk, token.pk, pk, pk)
			}

			query, args := token.Filter()
			if query != "id < ?" {
				t.Fatalf("filter query = %q, want %q", query, "id < ?")
			}
			if len(args) != 1 || args[0] != pk {
				t.Fatalf("filter args = %v, want [%v]", args, pk)
			}
		})
	}
}

type testInt64PK int64

func TestDefaultTokenNamedPK(t *testing.T) {
	raw := NewDefaultToken("id", testInt64PK(42))

	decoded, err := DefaultTokenDecoder("id")(raw.Encode())
	if err != nil {
		t.Fatalf("decode token: %v", err)
	}

	token := decoded.(DefaultToken)
	if token.pk != int64(42) {
		t.Fatalf("decoded pk = %v(%T), want int64(42)", token.pk, token.pk)
	}
}

func TestDefaultTokenUnsupportedPK(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("NewDefaultToken with struct pk: want panic")
		}
	}()

	NewDefaultToken("id", struct{}{})
}

// testCustomToken 演示业务如何实现一个自定义排序的游标
type testCustomToken struct {
	pk int64
}

func (t testCustomToken) Encode() string {
	return EncodeToken("custom_asc", 1, []byte{0, 0, 0, 0, 0, 0, 0, byte(t.pk)})
}

func (t testCustomToken) Filter() (string, []any) {
	return "id > ?", []any{t.pk}
}

func TestCustomToken(t *testing.T) {
	token := testCustomToken{pk: 42}

	query, args := token.Filter()
	if query != "id > ?" || len(args) != 1 || args[0].(int64) != 42 {
		t.Fatalf("Filter() = (%q, %v), want (%q, [42])", query, args, "id > ?")
	}

	if got := NextPageToken(token, 10, 10); got != token.Encode() {
		t.Fatalf("NextPageToken(custom) = %q, want %q", got, token.Encode())
	}

	if !strings.HasPrefix(token.Encode(), "custom_asc.1.") {
		t.Fatalf("custom token = %q, want custom_asc.1.* prefix", token.Encode())
	}
}

func TestDefaultTokenKindMismatch(t *testing.T) {
	custom := EncodeToken("custom_asc", 1, []byte{0, 0, 0, 0, 0, 0, 0, 1})

	if _, err := DefaultTokenDecoder("id")(custom); !errors.Is(err, ErrInvalidPageToken) {
		t.Fatalf("decode custom token with default decoder: err = %v, want ErrInvalidPageToken", err)
	}
}

func TestDecodeTokenInvalid(t *testing.T) {
	valid := NewDefaultToken("id", 10).Encode()
	parts := strings.SplitN(valid, ".", 3)
	payload := parts[2]

	tests := map[string]string{
		"empty":           "",
		"missing_parts":   "id_desc",
		"missing_payload": "id_desc.1",
		"wrong_version":   "id_desc.2." + payload,
		"wrong_kind":      "other.1." + payload,
		"bad_base64":      "id_desc.1.!!!",
		"too_long":        strings.Repeat("a", maxPageTokenLen+1),
	}

	for name, token := range tests {
		t.Run(name, func(t *testing.T) {
			if _, err := DecodeToken(defaultTokenKind, defaultTokenVersion, token); !errors.Is(err, ErrInvalidPageToken) {
				t.Fatalf("DecodeToken(%q) err = %v, want ErrInvalidPageToken", token, err)
			}
		})
	}
}

func TestNextPageToken(t *testing.T) {
	last := NewDefaultToken("id", 100)

	if got := NextPageToken(last, 10, 10); got != last.Encode() {
		t.Errorf("NextPageToken(last, 10, 10) = %q, want %q", got, last.Encode())
	}
	if got := NextPageToken(last, 9, 10); got != "" {
		t.Errorf("NextPageToken(last, 9, 10) = %q, want empty", got)
	}
	if got := NextPageToken(last, 0, 10); got != "" {
		t.Errorf("NextPageToken(last, 0, 10) = %q, want empty", got)
	}
	if got := NextPageToken(last, 10, 0); got != "" {
		t.Errorf("NextPageToken(last, 10, 0) = %q, want empty", got)
	}
	if got := NextPageToken(nil, 10, 10); got != "" {
		t.Errorf("NextPageToken(nil, 10, 10) = %q, want empty", got)
	}
}

func newTestSession(t *testing.T) *Session {
	t.Helper()

	engine, err := xorm.NewEngine("mysql", "user:pass@tcp(127.0.0.1:1)/test")
	if err != nil {
		t.Fatalf("new xorm engine: %v", err)
	}
	t.Cleanup(func() { _ = engine.Close() })

	return New(engine.NewSession())
}

func TestTokenLimitWithoutPagination(t *testing.T) {
	s := &Session{}

	if got, err := s.DefaultTokenLimit(nil, "id"); err != nil || got != s {
		t.Fatalf("DefaultTokenLimit(nil) = (%v, %v), want (%v, nil)", got, err, s)
	}
	if got, err := s.TokenLimitWith(testTokenPager{size: 0, token: "bad"}, DefaultTokenDecoder("id")); err != nil || got != s {
		t.Fatalf("TokenLimitWith(page_size=0) = (%v, %v), want (%v, nil)", got, err, s)
	}
}

func TestTokenLimitInvalidToken(t *testing.T) {
	s := &Session{}

	if _, err := s.TokenLimitWith(testTokenPager{size: 10, token: "bad"}, DefaultTokenDecoder("id")); !errors.Is(err, ErrInvalidPageToken) {
		t.Fatalf("TokenLimitWith(invalid token) err = %v, want ErrInvalidPageToken", err)
	}
	if _, err := s.TokenLimitWith(testTokenPager{size: 10, token: "bad"}, nil); !errors.Is(err, ErrInvalidPageToken) {
		t.Fatalf("TokenLimitWith(nil decoder) err = %v, want ErrInvalidPageToken", err)
	}
}

func TestTokenLimitValidToken(t *testing.T) {
	s := newTestSession(t)

	got, err := s.
		Cond(true, "id > ?", 0).
		DefaultTokenLimit(testTokenPager{
			size:  10,
			token: NewDefaultToken("id", 100).Encode(),
		}, "id")
	if err != nil {
		t.Fatalf("DefaultTokenLimit() err = %v, want nil", err)
	}
	if got != s {
		t.Fatalf("DefaultTokenLimit() = %v, want %v", got, s)
	}
}

func TestDeprecatedPageToken(t *testing.T) {
	if got := DecodePageToken(EncodePageToken(42)); got != 42 {
		t.Fatalf("DecodePageToken(EncodePageToken(42)) = %d, want 42", got)
	}
	if got := DecodePageToken("!!!"); got != 0 {
		t.Fatalf("DecodePageToken(!!!) = %d, want 0", got)
	}
}
