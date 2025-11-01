package util

// Mask masks a secret by keeping last 4 chars.
func Mask(s string) string {
    if s == "" { return "" }
    if len(s) <= 4 { return "****" }
    return "****" + s[len(s)-4:]
}

