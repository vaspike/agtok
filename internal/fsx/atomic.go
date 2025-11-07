package fsx

import (
    "fmt"
    "io/fs"
    "os"
    "path/filepath"
    "runtime"
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
    // Try rename, with Windows-specific retries and replacement fallback.
    if err := os.Rename(tmp, path); err != nil {
        // On Windows, rename fails if destination exists or is locked. Try limited retries.
        if runtime.GOOS == "windows" {
            // If destination exists, attempt replace by removing existing file then renaming.
            // Also retry a few times to get past transient locks.
            var last error = err
            for i := 0; i < 5; i++ {
                // If target exists, try remove and rename
                if _, statErr := os.Stat(path); statErr == nil {
                    _ = os.Remove(path)
                }
                if rerr := os.Rename(tmp, path); rerr == nil {
                    return nil
                } else {
                    last = rerr
                }
                time.Sleep(50 * time.Millisecond)
            }
            // Cleanup tmp on failure
            _ = os.Remove(tmp)
            return last
        }
        // Non-Windows: best effort cleanup and return original error
        _ = os.Remove(tmp)
        return err
    }
    return nil
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
