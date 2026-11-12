package backup

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestManagedRotationAndPathSafety(t *testing.T) {
	dir := t.TempDir()
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	for i := 0; i < 6; i++ {
		if err := SaveManaged(dir, []byte("backup"), start.Add(time.Duration(i)*time.Hour), 2, 3, time.Hour); err != nil {
			t.Fatal(err)
		}
	}
	files, err := ListManaged(dir)
	if err != nil {
		t.Fatal(err)
	}
	counts := map[string]int{}
	for _, f := range files {
		counts[f.Kind]++
		if _, err := ReadManaged(dir, f.Name); err != nil {
			t.Fatal(err)
		}
	}
	if counts["daily"] != 2 || counts["weekly"] != 3 {
		t.Fatal(counts)
	}
	if _, err := ReadManaged(dir, "../private.yaml"); err == nil {
		t.Fatal("traversal accepted")
	}
	if err := os.WriteFile(filepath.Join(dir, "unmanaged.yaml"), []byte("private"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadManaged(dir, "unmanaged.yaml"); err == nil {
		t.Fatal("unmanaged file accepted")
	}
	if _, err := os.Stat(filepath.Join(dir, "unmanaged.yaml")); err != nil {
		t.Fatal("rotation removed unmanaged file")
	}
}
