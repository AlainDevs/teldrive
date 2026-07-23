package http_range

import (
	"errors"
	"strconv"
	"strings"
)

type Range struct {
	Start int64
	End   int64
}

var (
	ErrNoOverlap = errors.New("invalid range: failed to overlap")

	ErrInvalid = errors.New("invalid range")
)

func Parse(header string, size int64) ([]*Range, error) {
	unit, values, ok := strings.Cut(strings.TrimSpace(header), "=")
	if !ok || !strings.EqualFold(strings.TrimSpace(unit), "bytes") || size <= 0 {
		return nil, ErrInvalid
	}

	arr := strings.Split(values, ",")
	ranges := make([]*Range, 0, len(arr))

	for _, value := range arr {
		startValue, endValue, ok := strings.Cut(strings.TrimSpace(value), "-")
		if !ok || strings.Contains(endValue, "-") {
			return nil, ErrInvalid
		}
		startValue = strings.TrimSpace(startValue)
		endValue = strings.TrimSpace(endValue)
		if startValue == "" && endValue == "" {
			return nil, ErrInvalid
		}

		var start, end int64
		if startValue == "" {
			var err error
			end, err = strconv.ParseInt(endValue, 10, 64)
			if err != nil || end <= 0 {
				return nil, ErrInvalid
			}
			if end > size {
				end = size
			}
			start = size - end
			end = size - 1
		} else {
			var err error
			start, err = strconv.ParseInt(startValue, 10, 64)
			if err != nil || start < 0 {
				return nil, ErrInvalid
			}
			if endValue == "" {
				end = size - 1
			} else {
				end, err = strconv.ParseInt(endValue, 10, 64)
				if err != nil || end < 0 || start > end {
					return nil, ErrInvalid
				}
			}
		}

		if end >= size {
			end = size - 1
		}

		if start >= size {
			return nil, ErrNoOverlap
		}

		ranges = append(ranges, &Range{
			Start: start,
			End:   end,
		})
	}

	if len(ranges) == 0 {
		return nil, ErrNoOverlap
	}

	return ranges, nil
}
