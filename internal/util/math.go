package util

import "golang.org/x/exp/constraints"

func Clamp[T constraints.Ordered](v, low, high T) T {
	return max(low, min(v, high))
}
