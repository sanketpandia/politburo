package flights

func Paginate[T any](items []T, pageNumber, pageLength int) []T {
	if items == nil {
		return make([]T, 0)
	}
	start := (pageNumber - 1) * pageLength
	if start < 0 || start >= len(items) {
		return make([]T, 0)
	}
	end := start + pageLength
	if end > len(items) {
		end = len(items)
	}
	page := make([]T, end-start)
	copy(page, items[start:end])
	return page
}
