// Package xmlutil provides shared XML utility functions.
package xmlutil

import "strings"

var xmlReplacer = strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", "\"", "&quot;", "'", "&apos;")

// Escape escapes XML special characters in a string.
func Escape(s string) string {
	return xmlReplacer.Replace(s)
}
