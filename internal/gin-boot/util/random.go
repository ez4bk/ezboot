// Copyright (C) 2019 LambdaCal Inc.

package util

import (
	"crypto/rand"
	"fmt"
)

const constCharCollections = "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"

// RandomData generates cryptographically secure random bytes.
func RandomData(byteNum int) ([]byte, error) {
	if byteNum < 0 {
		return nil, fmt.Errorf("random: negative length %d", byteNum)
	}

	data := make([]byte, byteNum)
	if _, err := rand.Read(data); err != nil {
		return nil, err
	}
	return data, nil
}

// RandomString returns a cryptographically secure random string of length
// strLen using the base62 alphabet.
func RandomString(strLen int) string {
	if strLen <= 0 {
		return ""
	}

	const maxByte = 248 // 248 == 62 * 4, rejection sampling avoids modulo bias
	out := make([]byte, strLen)
	buf := make([]byte, strLen)

	for i := 0; i < strLen; {
		if _, err := rand.Read(buf); err != nil {
			panic(fmt.Sprintf("random: %v", err))
		}
		for _, b := range buf {
			if b >= maxByte {
				continue
			}
			out[i] = constCharCollections[int(b)%len(constCharCollections)]
			i++
			if i == strLen {
				break
			}
		}
	}
	return string(out)
}
