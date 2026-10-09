package exp

func AsDefault[T comparable](a, def T) T {
	var zero T
	if zero == a {
		return def
	}
	return a
}
