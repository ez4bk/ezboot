package exp

func FirstOf[T1 any, T2 any](v1 T1, _ T2) T1 {
	return v1
}

func SecondOf[T1 any, T2 any](_ T1, v2 T2) T2 {
	return v2
}
