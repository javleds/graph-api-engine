package domain

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
	"time"
)

func ParseDuration(value string) (time.Duration, error) {
	parts := regexp.MustCompile(`^([1-9][0-9]*)(ms|s|m|h|d)$`).FindStringSubmatch(value)
	if parts == nil {
		return 0, fmt.Errorf("duration must be a positive integer and ms, s, m, h or d")
	}
	count, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		return 0, fmt.Errorf("duration overflows")
	}
	unit := map[string]time.Duration{"ms": time.Millisecond, "s": time.Second, "m": time.Minute, "h": time.Hour, "d": 24 * time.Hour}[parts[2]]
	if count > math.MaxInt64/int64(unit) {
		return 0, fmt.Errorf("duration overflows")
	}
	return time.Duration(count) * unit, nil
}
