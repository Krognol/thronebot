package router

import (
	"encoding/csv"
	"strings"
)

const separator = ' '

// Args is the command arguments
type Args []string

// Get gets it Nth argument
func (a Args) Get(n int) string {
	if n >= 0 && n < len(a) {
		return a[n]
	}
	return ""
}

// After returns the combined arguments after N
func (a Args) After(n int) string {
	if n >= 0 && n < len(a) {
		return strings.Join(a[n:], string(separator))
	}
	return ""
}

// ParseArgs ...
func ParseArgs(content string) Args {
	cv := csv.NewReader(strings.NewReader(content))
	cv.Comma = rune(separator)
	fields, err := cv.Read()
	if err != nil {
		return strings.Split(content, string(separator))
	}
	return fields
}
