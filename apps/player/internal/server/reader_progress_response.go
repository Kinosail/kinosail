package server

func readerProgressResponse(page, total int, offset float64, includeOffset bool) map[string]any {
	if page < 1 || page > total {
		page, offset = 1, 0
	}
	result := map[string]any{"page": page, "total": total}
	if includeOffset {
		result["offset"] = offset
	}
	return result
}
