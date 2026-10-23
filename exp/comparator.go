package exp

import "slices"

func In[T comparable](array []T, v T) bool {
	return slices.Contains(array, v)
}

func Max[T Ordered](a, b T) T {
	if a > b {
		return a
	}
	return b
}

func Min[T Ordered](a, b T) T {
	if a < b {
		return a
	}
	return b
}

func Abs[T Numeric](v T) T {
	if v > 0 {
		return v
	}
	return -v
}
