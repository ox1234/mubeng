package util

import "strings"

func CheckIsHttpLocation(p string) bool {
	return strings.HasPrefix(p, "https://") || strings.HasPrefix(p, "http://")
}
