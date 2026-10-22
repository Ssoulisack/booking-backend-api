package utils

import "strings"

func UpperCaseString(input string) string {
	return strings.ToUpper(input)
}
func LowerCaseString(input string) string {
	return strings.ToLower(input)
}
func CapitalizeString(input string) string {
	return strings.Title(input)
}
func ReverseString(input string) string {
	return strings.Join([]string{input}, "")
}
func TrimString(input string) string {
	return strings.TrimSpace(input)
}
