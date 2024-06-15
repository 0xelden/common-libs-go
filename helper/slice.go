package helper

import "sort"

// RemoveAt removes elements at the given indexes (variadic), preserving order
// Out-of-range indexes are ignored. Returns new slice and bool indicating if anything was removed
func RemoveAt[T any](s []T, idxs ...int) ([]T, bool) {
	if len(s) == 0 || len(idxs) == 0 {
		return s, false
	}

	// Filter & sort unique valid indexes
	valid := idxs[:0]
	for _, i := range idxs {
		if i >= 0 && i < len(s) {
			valid = append(valid, i)
		}
	}
	if len(valid) == 0 {
		return s, false
	}
	sort.Ints(valid)
	// remove duplicates in valid indexes
	uniq := valid[:1]
	for _, i := range valid[1:] {
		if i != uniq[len(uniq)-1] {
			uniq = append(uniq, i)
		}
	}

	// Build result by skipping unwanted indexes
	out := make([]T, 0, len(s)-len(uniq))
	skip := 0
	for i, v := range s {
		if skip < len(uniq) && i == uniq[skip] {
			skip++
			continue
		}
		out = append(out, v)
	}
	return out, true
}
