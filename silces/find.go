package silces

// Contains 判断切片是否包含目标元素。
// 注意：T 必须是 comparable 的。
func Contains[T comparable](collection []T, target T) bool {
	// TODO: 实现
	return false
}

// Find 查找第一个满足条件的元素。
// 返回值：(元素值, 是否找到)
func Find[T any](collection []T, predicate func(T) bool) (T, bool) {
	// TODO: 实现
	var zero T
	return zero, false
}
