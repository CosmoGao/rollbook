package backup

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

// 只接受本包生成的文件名，拒绝目录穿越。
var managedName = regexp.MustCompile(`^rollbook-(daily|weekly)-[0-9]{8}T[0-9]{6}\.[0-9]{9}Z\.yaml$`)

type ManagedFile struct {
	Name      string `json:"name"`
	Kind      string `json:"kind"`
	CreatedAt string `json:"created_at"`
	Size      int64  `json:"size"`
}

func Directory(ctx context.Context, db *sql.DB) (string, error) {
	if dir := os.Getenv("ROLLBOOK_BACKUP_DIR"); dir != "" {
		return dir, nil
	}
	rows, err := db.QueryContext(ctx, "PRAGMA database_list")
	if err != nil {
		return "", err
	}
	defer rows.Close()
	for rows.Next() {
		var seq int
		var name, path string
		if err := rows.Scan(&seq, &name, &path); err != nil {
			return "", err
		}
		if name == "main" && path != "" {
			return filepath.Dir(path), nil
		}
	}
	if err := rows.Err(); err != nil {
		return "", err
	}
	return "", ErrInvalid
}

func ListManaged(dir string) ([]ManagedFile, error) {
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return []ManagedFile{}, nil
	}
	if err != nil {
		return nil, err
	}
	files := []ManagedFile{}
	for _, entry := range entries {
		name := entry.Name()
		if !managedName.MatchString(name) || !entry.Type().IsRegular() {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			return nil, err
		}
		kind := "daily"
		if strings.HasPrefix(name, "rollbook-weekly-") {
			kind = "weekly"
		}
		stamp, err := time.Parse("20060102T150405.000000000Z", name[len("rollbook-"+kind+"-"):len(name)-len(".yaml")])
		if err != nil {
			continue
		}
		files = append(files, ManagedFile{Name: name, Kind: kind, CreatedAt: stamp.UTC().Format(time.RFC3339Nano), Size: info.Size()})
	}
	sort.Slice(files, func(i, j int) bool { return files[i].Name > files[j].Name })
	return files, nil
}

func ReadManaged(dir, name string) ([]byte, error) {
	if !managedName.MatchString(name) {
		return nil, ErrInvalid
	}
	files, err := ListManaged(dir)
	if err != nil {
		return nil, err
	}
	for _, file := range files {
		if file.Name == name {
			if file.Size > MaxSize {
				return nil, ErrInvalid
			}
			return os.ReadFile(filepath.Join(dir, name))
		}
	}
	return nil, ErrInvalid
}

func SaveManaged(dir string, data []byte, now time.Time, dailyKeep, weeklyKeep int, weeklyInterval time.Duration) error {
	if dailyKeep < 1 || weeklyKeep < 1 {
		return fmt.Errorf("%w: retention must be positive", ErrInvalid)
	}
	if weeklyInterval <= 0 {
		return ErrInvalid
	}
	prior, err := ListManaged(dir)
	if err != nil {
		return err
	}
	weeklyDue := true
	for _, file := range prior {
		if file.Kind == "weekly" {
			created, err := time.Parse(time.RFC3339Nano, file.CreatedAt)
			if err == nil && now.UTC().Sub(created) < weeklyInterval {
				weeklyDue = false
			}
			break
		}
	}
	stamp := ""
	for {
		stamp = now.UTC().Format("20060102T150405.000000000Z")
		_, err := os.Lstat(filepath.Join(dir, "rollbook-daily-"+stamp+".yaml"))
		if os.IsNotExist(err) {
			break
		}
		if err != nil {
			return err
		}
		now = now.Add(time.Nanosecond)
	}
	for _, kind := range []string{"daily", "weekly"} {
		if kind == "weekly" && !weeklyDue {
			continue
		}
		name := "rollbook-" + kind + "-" + stamp + ".yaml"
		if err := WriteAtomic(filepath.Join(dir, name), data); err != nil {
			return err
		}
	}
	files, err := ListManaged(dir)
	if err != nil {
		return err
	}
	seen := map[string]int{}
	for _, file := range files {
		seen[file.Kind]++
		limit := dailyKeep
		if file.Kind == "weekly" {
			limit = weeklyKeep
		}
		if seen[file.Kind] > limit {
			if err := os.Remove(filepath.Join(dir, file.Name)); err != nil {
				return err
			}
		}
	}
	return nil
}
