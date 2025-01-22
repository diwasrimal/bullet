package main

import (
	"fmt"
	"os"
)

// Prints to [os.Stderr]
func eprintf(format string, a ...any) {
	fmt.Fprintf(os.Stderr, format, a...)
}

func readableSize(b int64) string {
	const unit = 1000
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f%cB", float64(b)/float64(div), "kMGTPE"[exp])
}

func dbgprintf(format string, a ...any) {
	if debugEnabled {
		fmt.Fprintf(os.Stderr, format, a...)
	}
}

func ifelse[T any](cond bool, a, b T) T {
	if cond {
		return a
	}
	return b
}
