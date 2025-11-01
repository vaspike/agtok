package fsx

import (
    "fmt"
    "io/fs"
    "os"
    "path/filepath"
    "time"
)

// AtomicWrite writes content to a temp file and renames it into place.
func AtomicWrite(path string, content []byte, mode fs.FileMode) error {
    dir := filepath.Dir(path)
    base := filepath.Base(path)
    if err := os.MkdirAll(dir, 0o700); err != nil {
        return err
    }
    tmp := filepath.Join(dir, "."+base+".tmp")
    if err := os.WriteFile(tmp, content, mode); err != nil {
        return err
    }
    f, _ := os.Open(tmp)
    if f != nil { _ = f.Sync(); _ = f.Close() }
    return os.Rename(tmp, path)
}

// BackupFile creates a timestamped .bak copy if the file exists.
func BackupFile(path string) error {
    if _, err := os.Stat(path); err != nil {
        return err
    }
    dir := filepath.Dir(path)
    base := filepath.Base(path)
    stamp := time.Now().Format("20060102-150405")
    bak := filepath.Join(dir, fmt.Sprintf("%s.%s.bak", base, stamp))
    b, err := os.ReadFile(path)
    if err != nil { return err }
    return os.WriteFile(bak, b, 0o600)
}

