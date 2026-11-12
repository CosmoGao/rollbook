package backup

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
)

type DifferenceCount struct {
	Added    int `json:"added"`
	Modified int `json:"modified"`
	Deleted  int `json:"deleted"`
}
type ChangePreview struct {
	ID       string `json:"id"`
	ObjectID string `json:"object_id"`
	Action   string `json:"action"`
	Reason   string `json:"reason"`
	State    string `json:"state"`
}

func (s Service) RestorePreview(ctx context.Context, target File) (Preview, string, error) {
	if target.Kind != "full" && target.Kind != "catalog" {
		return Preview{}, "", ErrInvalid
	}
	data, err := s.Export(ctx, target.Kind)
	if err != nil {
		return Preview{}, "", err
	}
	current, _, err := Parse(data)
	if err != nil {
		return Preview{}, "", err
	}
	p := Preview{Kind: target.Kind, Counts: map[string]int{}, Differences: map[string]DifferenceCount{}, ObjectDifferences: map[string]DifferenceCount{}, Objects: []ObjectPreview{}, LostChanges: []ChangePreview{}}
	oldObjects := indexRows(current.Tables["objects"])
	newObjects := indexRows(target.Tables["objects"])
	actions := map[string]string{}
	names, _ := tables(target.Kind)
	for _, name := range names {
		before := indexRows(current.Tables[name])
		after := indexRows(target.Tables[name])
		counts := DifferenceCount{}
		p.Counts[name] = len(target.Tables[name])
		for id, row := range after {
			old, exists := before[id]
			action := ""
			if !exists {
				counts.Added++
				action = "add"
			} else if !sameRow(old, row) {
				counts.Modified++
				action = "modify"
			}
			if action != "" && (oldObjects[id] != nil || newObjects[id] != nil) {
				actions[id] = action
			}
		}
		for id := range before {
			if _, exists := after[id]; !exists {
				counts.Deleted++
				if oldObjects[id] != nil || newObjects[id] != nil {
					actions[id] = "delete"
				}
			}
		}
		p.Differences[name] = counts
	}
	for id, action := range actions {
		row := newObjects[id]
		if action == "delete" {
			row = oldObjects[id]
		}
		if row == nil {
			continue
		}
		typ := fmt.Sprint(row["type"])
		p.Objects = append(p.Objects, ObjectPreview{ID: id, Type: typ, Action: action})
		count := p.ObjectDifferences[typ]
		switch action {
		case "add":
			count.Added++
		case "modify":
			count.Modified++
		case "delete":
			count.Deleted++
		}
		p.ObjectDifferences[typ] = count
	}
	for id, row := range indexRows(current.Tables["changes"]) {
		other, exists := indexRows(target.Tables["changes"])[id]
		if !exists || !sameRow(row, other) {
			p.LostChanges = append(p.LostChanges, ChangePreview{ID: id, ObjectID: fmt.Sprint(row["object_id"]), Action: fmt.Sprint(row["action"]), Reason: fmt.Sprint(row["reason"]), State: fmt.Sprint(row["state"])})
		}
	}
	sort.Slice(p.Objects, func(i, j int) bool { return p.Objects[i].ID < p.Objects[j].ID })
	sort.Slice(p.LostChanges, func(i, j int) bool { return p.LostChanges[i].ID < p.LostChanges[j].ID })
	encoded, _ := json.Marshal(struct {
		Preview Preview                     `json:"preview"`
		Current map[string][]map[string]any `json:"current"`
	}{p, current.Tables})
	hash := sha256.Sum256(encoded)
	return p, hex.EncodeToString(hash[:]), nil
}
func indexRows(rows []map[string]any) map[string]map[string]any {
	result := make(map[string]map[string]any, len(rows))
	for _, row := range rows {
		result[fmt.Sprint(row["id"])] = row
	}
	return result
}
func sameRow(a, b map[string]any) bool {
	x, _ := json.Marshal(a)
	y, _ := json.Marshal(b)
	return string(x) == string(y)
}
