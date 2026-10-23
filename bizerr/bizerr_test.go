package bizerr

import (
	"errors"
	"net/http"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/ez4bk/ezboot/internal/pberr"
)

func TestBizErrorUsesStandardGRPCCodeAndBizDetail(t *testing.T) {
	err := New(10001, http.StatusBadRequest, "bad text")
	st := err.GRPCStatus()

	if st.Code() != codes.InvalidArgument {
		t.Fatalf("GRPCStatus code = %v, want %v", st.Code(), codes.InvalidArgument)
	}

	var gotBizCode uint32
	var gotHTTPCode uint32
	for _, detail := range st.Details() {
		switch v := detail.(type) {
		case *pberr.WithBizCode:
			gotBizCode = v.BizCode
		case *pberr.WithHttpStatus:
			gotHTTPCode = v.HttpCode
		}
	}
	if gotBizCode != 10001 {
		t.Fatalf("biz code detail = %d, want 10001", gotBizCode)
	}
	if gotHTTPCode != http.StatusBadRequest {
		t.Fatalf("http code detail = %d, want %d", gotHTTPCode, http.StatusBadRequest)
	}
}

func TestBizErrorWithGRPCCode(t *testing.T) {
	err := New(10001, http.StatusConflict, "conflict").
		WithGRPCCode(codes.FailedPrecondition)

	if got := err.GRPCStatus().Code(); got != codes.FailedPrecondition {
		t.Fatalf("GRPCStatus code = %v, want %v", got, codes.FailedPrecondition)
	}
}

func TestFromErrorRoundTrip(t *testing.T) {
	cause := errors.New("database unavailable")
	original := New(10001, http.StatusConflict, "conflict").
		WithGRPCCode(codes.FailedPrecondition).
		WithCause(cause)

	decoded, ok := FromError(original)
	if !ok {
		t.Fatal("FromError() = false, want true")
	}
	if decoded.Code() != 10001 {
		t.Fatalf("biz code = %d, want 10001", decoded.Code())
	}
	if decoded.HttpCode() != http.StatusConflict {
		t.Fatalf("http code = %d, want %d", decoded.HttpCode(), http.StatusConflict)
	}
	if decoded.GRPCCode() != codes.FailedPrecondition {
		t.Fatalf("grpc code = %v, want %v", decoded.GRPCCode(), codes.FailedPrecondition)
	}
	if decoded.Unwrap() == nil || decoded.Unwrap().Error() != cause.Error() {
		t.Fatalf("decoded cause = %v, want %q", decoded.Unwrap(), cause)
	}
}

func TestFromErrorRejectsPlainError(t *testing.T) {
	if _, ok := FromError(errors.New("plain")); ok {
		t.Fatal("FromError(plain error) = true, want false")
	}
}

func TestBizErrorIsComparesBusinessCode(t *testing.T) {
	a := New(10001, http.StatusBadRequest, "a")
	b := New(10001, http.StatusConflict, "b")
	c := New(10002, http.StatusBadRequest, "c")

	if !errors.Is(a, b) {
		t.Fatal("errors.Is(a, b) = false, want true for same biz code")
	}
	if errors.Is(a, c) {
		t.Fatal("errors.Is(a, c) = true, want false for different biz code")
	}
}

func TestBizErrorUnwrap(t *testing.T) {
	cause := errors.New("cause")
	err := New(10001, http.StatusBadRequest, "bad").WithCause(cause)

	if !errors.Is(err, cause) {
		t.Fatal("errors.Is(err, cause) = false")
	}
}

func TestFromErrorKeepsLegacyHTTPDetail(t *testing.T) {
	// Old peers may only send WithHttpStatus, without WithBizCode.
	errorStatus, err := status.New(codes.InvalidArgument, "legacy").
		WithDetails(&pberr.WithHttpStatus{HttpCode: http.StatusUnprocessableEntity})
	if err != nil {
		t.Fatalf("WithDetails() = %v", err)
	}

	decoded, ok := FromError(errorStatus.Err())
	if !ok {
		t.Fatal("FromError(legacy) = false, want true")
	}
	if decoded.Code() != uint32(codes.InvalidArgument) {
		t.Fatalf("legacy biz code = %d, want %d", decoded.Code(), codes.InvalidArgument)
	}
	if decoded.HttpCode() != http.StatusUnprocessableEntity {
		t.Fatalf("legacy http code = %d, want %d", decoded.HttpCode(), http.StatusUnprocessableEntity)
	}
}
