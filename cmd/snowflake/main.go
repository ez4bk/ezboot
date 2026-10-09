package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/ez4bk/ezboot/snowflake"
)

const (
	PaddingLength = 20
	PaddingBlank  = " "
	PaddingWave   = "~"
)

func AlignCenter(v any, l int, pad string) string {
	s := fmt.Sprintf("%v", v)

	if len(s) >= l {
		return s
	}

	prefix := strings.Repeat(pad, (l-len(s))/2)
	return prefix + s + strings.Repeat(pad, l-len(s)-len(prefix))
}

func IfPresent(header string, f func(io.Writer)) {
	var buf bytes.Buffer
	if f(&buf); buf.Len() != 0 {
		fmt.Println(header)
		fmt.Println(buf.String())
	}
}

func main() {
	if len(os.Args) == 1 {
		_, _ = fmt.Fprintf(os.Stderr, "usage: snowflake [ID, +ID, -ID]\n\n")
		return
	}

	numberHeader := AlignCenter(" Number ", PaddingLength, PaddingWave)
	snowflakeHeader := AlignCenter(" SnowflakeID ", PaddingLength, PaddingWave)

	ire := regexp.MustCompile("^\\d+$")
	IfPresent(fmt.Sprintf("%s <> %s", numberHeader, snowflakeHeader), func(w io.Writer) {
		for _, arg := range os.Args[1:] {
			if ire.MatchString(arg) {
				if n, err := strconv.Atoi(arg); err == nil {
					_, _ = fmt.Fprintf(w, "%s => %s\n",
						AlignCenter(n, PaddingLength, PaddingBlank),
						AlignCenter(snowflake.ID(n).String(), PaddingLength, PaddingBlank),
					)
				}
			}
		}
	})

	sre := regexp.MustCompile("^[+-]?(.+)$")
	IfPresent(fmt.Sprintf("%s <> %s", snowflakeHeader, numberHeader), func(w io.Writer) {
		for _, arg := range os.Args[1:] {
			if !ire.MatchString(arg) {
				if matches := sre.FindStringSubmatch(arg); len(matches) != 0 {
					if n, err := snowflake.ParseString(matches[1]); err == nil {
						_, _ = fmt.Fprintf(w, "%s => %s\n",
							AlignCenter(matches[1], PaddingLength, PaddingBlank),
							AlignCenter(n.Int64(), PaddingLength, PaddingBlank))
					}
				}
			}
		}
	})
}
