package helper

/*
Map applies the mapper function to every element of the source slice and returns
a *new* slice containing the transformed values, in the same order.

  - source parameter: the input slice you want to transform.
  - mapper function parameter: receives (value, index) and returns the mapped
    value for that position.

If mapper is nil it panics.
*/
func Map[T1 any, T2 any](source []T1, mapper func(value T1, index int) T2) []T2 {
	if mapper == nil {
		panic("Map: mapper func must not be nil")
	}
	output := make([]T2, len(source))
	for i, v := range source {
		output[i] = mapper(v, i)
	}
	return output
}

/*
Reduce runs the reducer function on each element of the array. In order, the reduce function passes in the return value from the calculation on the preceding element. The final result of running the reducer across all elements of the array is the return value of the final reducer on the last element.
- list paramerter: source list we want to process.
- initial value paramerter: the previous value that's used in the reducer call of the first element. At this time, previous = initial value, current = first element of the list.
- reducer function paramerter: the function will run on all elements of the source list.
*/
func Reduce[T1 any, T2 any](source []T1, initialValue T2, reducer func(result T2, value T1, index int) T2) T2 {
	previousItem := initialValue
	output := initialValue
	for index := range source {
		output = reducer(previousItem, source[index], index)
		previousItem = output
	}
	return output
}

// Coalesce returns the first non-zero value from listed arguments.
// Returns the zero value of the type parameter if no arguments are given or all are the zero value.
// Useful when you want to initialize a variable to the first non-zero value from a list of fallback values.
//
// For example:
//
//	hostVal := Coalesce(hostName, os.Getenv("HOST"), "localhost")
func Coalesce[T comparable](values ...T) (v T) {
	var zero T
	for _, v = range values {
		if v != zero {
			return
		}
	}
	return
}

// If returns vtrue if cond is true, vfalse otherwise.
//
// Useful to avoid an if statement when initializing variables, for example:
//
//	min := If(i > 0, i, 0)
func If[T any](cond bool, vtrue, vfalse T) T {
	if cond {
		return vtrue
	}
	return vfalse
}

// First returns the first argument.
// Useful when you want to use the first result of a function call that has more than one return values
// (e.g. in a composite literal or in a condition).
//
// For example:
//
//	func f() (i, j, k int, s string, f float64) { return }
//
//	p := image.Point{
//	    X: First(f()),
//	}
func First[T any](first T, _ ...any) T {
	return first
}

// Second returns the second argument.
// Useful when you want to use the second result of a function call that has more than one return values
// (e.g. in a composite literal or in a condition).
func Second[T any](_ any, second T, _ ...any) T {
	return second
}

// Ptr returns a pointer to the passed value.
//
// Useful when you have a value and need a pointer, e.g.:
//
//	func f() string { return "foo" }
//
//	foo := struct{
//	    Bar *string
//	}{
//	    Bar: Ptr(f()),
//	}
func Ptr[T any](v T) *T {
	return &v
}

// Deref safely dereferences a pointer of any type to its underlying type.
func Deref[T any](ptr *T) (res T) {
	if ptr == nil {
		return res
	}
	// nosemgrep
	return *ptr
}

// Filter returns a new slice holding only the filtered elements.
func Filter[S ~[]E, E any](vals S, f func(v E) bool) S {
	var out S
	for _, v := range vals {
		if f(v) {
			out = append(out, v)
		}
	}
	return out
}

// Index "safely" indexes a slice.
// If the index is invalid (out of range) for the given slice, the (first) def is returned.
// If def is not specified, the zero value of E is returned.
func Index[E any](vals []E, idx int, def ...E) (result E) {
	if idx >= 0 && idx < len(vals) {
		return vals[idx]
	}
	if len(def) > 0 {
		return def[len(def)-1]
	}
	return
}

// FindValue function takes a slice of any type T, and a function that processes elements of that type T
// and returns a value of type R and a boolean indicating success
func FindValue[T any, R any](slice []T, processor func(T) (R, bool)) R {
	var zero R
	for _, elem := range slice {
		if result, ok := processor(elem); ok {
			return result
		}
	}
	return zero
}

// MapToSlice function takes a map with string keys and values of any type T and returns a slice of T
func MapToSlice[T any](inputMap map[string]T) []T {
	result := make([]T, len(inputMap)) // Pre-allocate the slice with the size of the input map
	i := 0
	for _, value := range inputMap {
		result[i] = value
		i++
	}
	return result
}

// MapKeys returns all keys from inputMap in an unspecified order
// If inputMap is nil or empty, it returns an empty slice
// The result slice is preallocated to len(inputMap)
func MapKeys[T any](inputMap map[string]T) []string {
	result := make([]string, 0, len(inputMap))
	for k := range inputMap {
		result = append(result, k)
	}
	return result
}

// MapFirstValue function takes a map with string keys and values of any type T
// and returns the first struct found and a boolean indicating if a struct was found
func MapFirstValue[T any](inputMap map[string]T) (T, bool) {
	var zero T
	for _, value := range inputMap {
		return value, true
	}
	return zero, false
}

// Distinct remove duplicate values from a slice
func Distinct[T comparable](slice []T) []T {
	maps := make(map[T]struct{}, len(slice))
	for _, v := range slice {
		maps[v] = struct{}{}
	}
	res := make([]T, 0, len(maps))
	for k := range maps {
		res = append(res, k)
	}
	return res
}

// DistinctBy munches a slice, projects each element, and spits out the
// first-seen uniques in order.
//
//	projects := helper.DistinctBy(param.Rows, func(r models.XlsxBomRow) string {
//	    return r.Project
//	})
//
// T  – original slice element type.
// R  – projected “key” type (must be comparable so we can stick it in a map).
func DistinctBy[T any, R comparable](items []T, project func(T) R) []R {
	if len(items) == 0 {
		return nil
	}

	seen := make(map[R]struct{}, len(items))
	result := make([]R, 0, len(items))

	for _, it := range items {
		key := project(it)
		if _, dup := seen[key]; dup {
			continue // already have it, move along.
		}
		seen[key] = struct{}{}
		result = append(result, key)
	}
	return result
}

// UniqueBy returns a new slice that contains only the first occurrence of each
// projected key in items, preserving the original order.
//
// The project function is used to derive a comparable key from each item.
// When multiple items produce the same key, only the first item is kept.
//
// If items is empty, UniqueBy returns nil.
func UniqueBy[T any, R comparable](items []T, project func(T) R) []T {
	if len(items) == 0 {
		return nil
	}

	seen := make(map[R]struct{}, len(items))
	result := make([]T, 0, len(items))

	for _, it := range items {
		key := project(it)
		if _, dup := seen[key]; dup {
			continue // already have it, move along.
		}
		seen[key] = struct{}{}
		result = append(result, it)
	}
	return result
}

// UniqueByMerge returns a new slice that contains only the first occurrence
// of each projected key in items, preserving the original order.
//
// project is used to derive a comparable key from each item.
// When multiple items produce the same key, merge is called with the
// current result item (existing) and the new duplicate item (dup) to
// compute the merged value that will be stored in the result slice.
//
// Example use-case: summing a field on duplicate keys.
//
// If items is empty, UniqueByMerge returns nil.
func UniqueByMerge[T any, R comparable](
	items []T,
	project func(T) R,
	merge func(existing, dup T) T,
) []T {
	if len(items) == 0 {
		return nil
	}

	// Map key -> index in result slice
	seen := make(map[R]int, len(items))
	result := make([]T, 0, len(items))

	for _, it := range items {
		key := project(it)
		if idx, ok := seen[key]; ok {
			// Merge into existing item at that index
			result[idx] = merge(result[idx], it)
			continue
		}

		seen[key] = len(result)
		result = append(result, it)
	}

	return result
}

// GroupBy returns an object composed of keys generated from the results of running each element of collection through iteratee.
// Play: https://go.dev/play/p/XnQBd_v6brd
func GroupBy[T any, U comparable, Slice ~[]T](collection Slice, iteratee func(item T) U) map[U]Slice {
	result := map[U]Slice{}

	for i := range collection {
		key := iteratee(collection[i])

		result[key] = append(result[key], collection[i])
	}

	return result
}

// SliceChunk splits a slice into chunks of the given size.
func SliceChunk[T any](slice []T, chunkSize int) [][]T {
	if chunkSize <= 0 {
		return [][]T{slice}
	}
	var chunks [][]T
	for i := 0; i < len(slice); i += chunkSize {
		end := i + chunkSize
		if end > len(slice) {
			end = len(slice)
		}
		chunks = append(chunks, slice[i:end])
	}
	return chunks
}

// KeyBy transforms a slice of type T into a map where each key is generated by the keyFn function.
// The type K must be comparable because it is used as a map key.
// Note: If multiple items produce the same key via keyFn, the later item will overwrite the previous one.
func KeyBy[T any, K comparable](items []T, keyFn func(T) K) map[K]T {
	result := make(map[K]T, len(items))
	for _, item := range items {
		result[keyFn(item)] = item
	}
	return result
}

// FirstNSlice returns at most n elements from the front of src.
// No panic; the underlying array is not copied unless you ask for it.
func FirstNSlice[T any](src []T, n int) []T {
	if n < 0 { // protect against negative n
		n = 0
	}
	if len(src) < n {
		n = len(src)
	}
	return src[:n]
}

// FirstNRunes returns a substring containing at most n Unicode code points.
// It never splits a multibyte rune, so the result is valid UTF-8.
func FirstNRunes(s string, n int) string {
	if n <= 0 {
		return ""
	}
	i := 0
	for idx := range s {
		if i == n { // idx is the byte index of rune #n
			return s[:idx]
		}
		i++
	}
	return s // s has ≤ n runes
}

// Or returns true if value equals any of the candidates.
// Useful to replace long `value == a || value == b` chains.
func Or[T comparable](value T, candidates ...T) bool {
	for _, candidate := range candidates {
		if value == candidate {
			return true
		}
	}
	return false
}
