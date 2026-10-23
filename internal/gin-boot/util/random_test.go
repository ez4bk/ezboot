package util

import (
	"strings"
	"testing"
)

func TestRandomStringLengthAndAlphabet(t *testing.T) {
	const alphabet = "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	got := RandomString(128)
	if len(got) != 128 {
		t.Fatalf("len(RandomString(128)) = %d", len(got))
	}
	for _, r := range got {
		if !strings.ContainsRune(alphabet, r) {
			t.Fatalf("RandomString() contains invalid rune %q", r)
		}
	}
	if RandomString(0) != "" {
		t.Fatal("RandomString(0) is not empty")
	}
}

func TestRandomDataLength(t *testing.T) {
	data, err := RandomData(32)
	if err != nil {
		t.Fatal(err)
	}
	if len(data) != 32 {
		t.Fatalf("len(RandomData(32)) = %d", len(data))
	}
	if _, err := RandomData(-1); err == nil {
		t.Fatal("RandomData(-1) succeeded")
	}
}
