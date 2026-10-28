// Package platforms maps birdROMs platform names to muOS ROM folders.
package platforms

import (
	_ "embed"
	"encoding/csv"
	"strings"
)

// Unsorted is the visible fallback folder for platforms with no known muOS
// folder. Files there are flagged in the UI so the user can move them.
const Unsorted = "_UNSORTED"

//go:embed platform_aliases.csv
var aliasesCSV string

// folders maps canonical platform names to muOS folder names, built once from
// the embedded alias table (a copy of the server's platform_aliases.csv). The
// platform string is only ever a lookup key; it is never joined onto a path.
var folders = mustLoadFolders(aliasesCSV)

// mustLoadFolders parses the alias table, keeping rows that name a muOS folder
// and are not non-game. The CSV is a build-time asset, so a malformed one
// panics at init rather than silently dropping platforms.
func mustLoadFolders(data string) map[string]string {
	rows, err := csv.NewReader(strings.NewReader(data)).ReadAll()
	if err != nil {
		panic("platforms: bad embedded platform_aliases.csv: " + err.Error())
	}
	if len(rows) == 0 {
		panic("platforms: empty embedded platform_aliases.csv")
	}
	col := map[string]int{}
	for i, name := range rows[0] {
		col[name] = i
	}
	for _, name := range []string{"canonical_platform", "muos_folder", "kind"} {
		if _, ok := col[name]; !ok {
			panic("platforms: platform_aliases.csv missing column " + name)
		}
	}
	m := make(map[string]string, len(rows))
	for _, row := range rows[1:] {
		dir := row[col["muos_folder"]]
		if dir == "" || row[col["kind"]] == "non-game" {
			continue
		}
		m[row[col["canonical_platform"]]] = dir
	}
	return m
}

// Folder returns the muOS folder name (a single path segment) for platform.
// Anything not in the table returns (Unsorted, false).
func Folder(platform string) (dir string, known bool) {
	if d, ok := folders[platform]; ok {
		return d, true
	}
	return Unsorted, false
}
