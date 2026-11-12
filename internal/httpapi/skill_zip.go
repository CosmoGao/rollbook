package httpapi

import (
	"archive/zip"
	"bytes"
	"io/fs"
	"net/http"
	"strconv"

	"github.com/CosmoGao/rollbook/skills"
)

func skillZip(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", "GET")
		WriteJSON(w, http.StatusMethodNotAllowed, APIError{Code: "method_not_allowed", Message: "Method not allowed"})
		return
	}
	var buf bytes.Buffer
	archive := zip.NewWriter(&buf)
	err := fs.WalkDir(skills.Rollbook, "rollbook", func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil || entry.IsDir() {
			return walkErr
		}
		data, err := fs.ReadFile(skills.Rollbook, path)
		if err != nil {
			return err
		}
		file, err := archive.Create(path)
		if err != nil {
			return err
		}
		_, err = file.Write(data)
		return err
	})
	if err == nil {
		err = archive.Close()
	}
	if err != nil {
		WriteJSON(w, 500, APIError{Code: "internal", Message: "Skill unavailable"})
		return
	}
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", `attachment; filename="rollbook-skill.zip"`)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Length", strconv.Itoa(buf.Len()))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(buf.Bytes())
}
