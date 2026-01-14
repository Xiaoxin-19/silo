package sliceutil

// Intersection returns the intersection of two slices.
func Intersection[T comparable](a, b []T) []T {
	if len(a) == 0 || len(b) == 0 {
		return []T{}
	}
	// BCE hint: avoid bounds check in loop
	_ = b[len(b)-1]
	mapB := make(map[T]struct{}, len(b))
	for _, v := range b {
		mapB[v] = struct{}{}
	}

	// Pre-allocate result. The size is at most the length of the smaller set, but len(a) is a safe upper bound estimate.
	capacity := min(len(b), len(a))
	result := make([]T, 0, capacity)
	_ = a[len(a)-1]
	for _, v := range a {
		if _, found := mapB[v]; found {
			result = append(result, v)
			delete(mapB, v) // ensure uniqueness in result
		}
	}
	return result
}

// IntersectionBy returns the intersection of two slices using a key selector.
// Useful for non-comparable types or custom uniqueness logic.
func IntersectionBy[T any, K comparable](a, b []T, keySelector func(T) K) []T {
	if len(a) == 0 || len(b) == 0 {
		return []T{}
	}
	_ = b[len(b)-1]
	mapB := make(map[K]struct{}, len(b))
	for _, v := range b {
		mapB[keySelector(v)] = struct{}{}
	}

	capacity := min(len(b), len(a))
	result := make([]T, 0, capacity)
	_ = a[len(a)-1]
	for _, v := range a {
		k := keySelector(v)
		if _, found := mapB[k]; found {
			result = append(result, v)
			delete(mapB, k)
		}
	}
	return result
}

// SymmetricDifference returns the symmetric difference of two slices.
// The result contains elements that are in either 'a' or 'b' but not in both.
func SymmetricDifference[T comparable](a, b []T) []T {
	if len(a) == 0 && len(b) == 0 {
		return []T{}
	}

	// Map to track state of elements.
	// true: present in b, candidate for result (if not found in a).
	// false: seen in a (either intersection or already added to result).
	bMap := make(map[T]bool, len(a)+len(b))

	if len(b) > 0 {
		_ = b[len(b)-1]
		for _, v := range b {
			bMap[v] = true
		}
	}

	result := make([]T, 0, len(a)+len(b))

	if len(a) > 0 {
		_ = a[len(a)-1]
		for _, v := range a {
			if inB, ok := bMap[v]; ok {
				if inB {
					// Found in B (Intersection). Mark as seen/exclude.
					bMap[v] = false
				}
			} else {
				// Not in B. Unique to A.
				result = append(result, v)
				// Mark as seen so duplicates in A are skipped
				bMap[v] = false
			}
		}
	}

	if len(b) > 0 {
		_ = b[len(b)-1]
		for _, v := range b {
			if keep, ok := bMap[v]; ok && keep {
				result = append(result, v)
				bMap[v] = false
			}
		}
	}

	return result
}

// SymmetricDifferenceBy returns the symmetric difference of two slices using a key selector.
// The result contains elements that are in either 'a' or 'b' but not in both.
func SymmetricDifferenceBy[T any, K comparable](a, b []T, keySelector func(T) K) []T {
	if len(a) == 0 && len(b) == 0 {
		return []T{}
	}

	bMap := make(map[K]bool, len(a)+len(b))

	if len(b) > 0 {
		_ = b[len(b)-1]
		for _, v := range b {
			bMap[keySelector(v)] = true
		}
	}

	result := make([]T, 0, len(a)+len(b))

	if len(a) > 0 {
		_ = a[len(a)-1]
		for _, v := range a {
			k := keySelector(v)
			if inB, ok := bMap[k]; ok {
				if inB {
					bMap[k] = false
				}
			} else {
				result = append(result, v)
				bMap[k] = false
			}
		}
	}

	if len(b) > 0 {
		_ = b[len(b)-1]
		for _, v := range b {
			k := keySelector(v)
			if keep, ok := bMap[k]; ok && keep {
				result = append(result, v)
				bMap[k] = false
			}
		}
	}

	return result
}

// Difference returns the difference between two slices (a - b).
// The result contains elements from 'a' that are not in 'b', with duplicates removed.
func Difference[T comparable](a, b []T) []T {
	if len(a) == 0 {
		return []T{}
	}

	// The map will eventually contain all unique elements from b and the unique elements from a that are added to result.
	// So len(a) + len(b) is a safe upper bound to avoid resizing.
	exclude := make(map[T]struct{}, len(a)+len(b))
	if len(b) > 0 {
		_ = b[len(b)-1]
		for _, v := range b {
			exclude[v] = struct{}{}
		}
	}

	result := make([]T, 0, len(a))
	_ = a[len(a)-1]
	for _, v := range a {
		if _, exists := exclude[v]; !exists {
			result = append(result, v)
			exclude[v] = struct{}{}
		}
	}
	return result
}

// DifferenceBy returns the difference between two slices using a key selector.
// The result contains elements from 'a' whose keys are not in 'b', with duplicates removed.
func DifferenceBy[T any, K comparable](a, b []T, keySelector func(T) K) []T {
	if len(a) == 0 {
		return []T{}
	}

	exclude := make(map[K]struct{}, len(a)+len(b))
	if len(b) > 0 {
		_ = b[len(b)-1]
		for _, v := range b {
			exclude[keySelector(v)] = struct{}{}
		}
	}

	result := make([]T, 0, len(a))
	_ = a[len(a)-1]
	for _, v := range a {
		k := keySelector(v)
		if _, exists := exclude[k]; !exists {
			result = append(result, v)
			exclude[k] = struct{}{}
		}
	}
	return result
}

// Unique removes duplicate elements from the slice in-place.
// It modifies the underlying array and returns the sliced result.
// The relative order of the first occurrence of elements is preserved.
func Unique[T comparable](collection []T) []T {
	if len(collection) == 0 {
		return collection
	}
	// BCE hint: avoid bounds check in loop
	_ = collection[len(collection)-1]

	seen := make(map[T]struct{}, len(collection))
	idx := 0
	for i, v := range collection {
		if _, ok := seen[v]; !ok {
			seen[v] = struct{}{}
			if i != idx {
				collection[idx] = v
			}
			idx++
		}
	}

	// Zero out remaining elements for GC
	clear(collection[idx:])
	return collection[:idx]
}

// UniqueBy removes duplicate elements from the slice in-place using a key selector.
// It modifies the underlying array and returns the sliced result.
// The relative order of the first occurrence of elements is preserved.
func UniqueBy[T any, K comparable](collection []T, keySelector func(T) K) []T {
	if len(collection) == 0 {
		return collection
	}
	_ = collection[len(collection)-1]

	seen := make(map[K]struct{}, len(collection))
	idx := 0
	for i, v := range collection {
		k := keySelector(v)
		if _, ok := seen[k]; !ok {
			seen[k] = struct{}{}
			if i != idx {
				collection[idx] = v
			}
			idx++
		}
	}

	clear(collection[idx:])
	return collection[:idx]
}

// Union returns the union of two slices (merged and deduplicated).
func Union[T comparable](a, b []T) []T {
	result := make([]T, 0, len(a)+len(b))

	seen := make(map[T]struct{}, len(a)+len(b))
	// BCE hint: avoid bounds check in loop
	if len(a) > 0 {
		_ = a[len(a)-1]
		for _, v := range a {
			if _, alreadyAdded := seen[v]; !alreadyAdded {
				result = append(result, v)
				seen[v] = struct{}{}
			}
		}
	}

	if len(b) > 0 {
		_ = b[len(b)-1]
		for _, v := range b {
			if _, alreadyAdded := seen[v]; !alreadyAdded {
				result = append(result, v)
				seen[v] = struct{}{}
			}
		}
	}
	return result
}

// UnionBy returns the union of two slices using a key selector.
// Useful for non-comparable types or custom uniqueness logic.
func UnionBy[T any, K comparable](a, b []T, keySelector func(T) K) []T {
	result := make([]T, 0, len(a)+len(b))
	seen := make(map[K]struct{}, len(a)+len(b))

	if len(a) > 0 {
		_ = a[len(a)-1]
		for _, v := range a {
			k := keySelector(v)
			if _, alreadyAdded := seen[k]; !alreadyAdded {
				result = append(result, v)
				seen[k] = struct{}{}
			}
		}
	}

	if len(b) > 0 {
		_ = b[len(b)-1]
		for _, v := range b {
			k := keySelector(v)
			if _, alreadyAdded := seen[k]; !alreadyAdded {
				result = append(result, v)
				seen[k] = struct{}{}
			}
		}
	}
	return result
}
