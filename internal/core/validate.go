package core

import (
    "errors"
    "net/url"
    "strings"
)

func ValidateFields(f Fields) error {
    if strings.TrimSpace(f.URL) == "" {
        return errors.New("url is required")
    }
    // Basic URL validation, allow http(s) and custom schemes
    u, err := url.Parse(f.URL)
    if err != nil || u.Scheme == "" || u.Host == "" {
        return errors.New("invalid url")
    }
    // token can be empty (e.g., only changing URL)
    return nil
}

