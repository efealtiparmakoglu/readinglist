package rl

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// Load reads a list from disk. A missing file yields an empty list.
func Load(path string) (List, error) {
	var list List
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return list, nil
	}
	if err != nil {
		return list, err
	}
	if len(data) == 0 {
		return list, nil
	}
	if err := json.Unmarshal(data, &list); err != nil {
		return list, err
	}
	if list.Next == 0 && len(list.Items) > 0 {
		// heal files written by an older version
		highest := 0
		for _, item := range list.Items {
			if item.ID > highest {
				highest = item.ID
			}
		}
		list.Next = highest + 1
	}
	return list, nil
}

// Save writes the list, creating the parent directory if needed.
func Save(path string, list List) error {
	if dir := filepath.Dir(path); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	data, err := json.MarshalIndent(list, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}
