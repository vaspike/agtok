package providers

import (
    "os"
    "path/filepath"
    "runtime"
)

func userHome() string {
    h, _ := os.UserHomeDir()
    if h == "" && runtime.GOOS == "windows" {
        h = os.Getenv("USERPROFILE")
    }
    return h
}

func joinHome(parts ...string) string {
    elems := append([]string{userHome()}, parts...)
    return filepath.Join(elems...)
}

