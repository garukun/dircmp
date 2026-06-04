package main

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type DirData struct {
	FilesByRelPath map[string]string   // RelPath -> Hash
	FilesByHash    map[string][]string // Hash -> []RelPath
	EmptyDirs      map[string]bool     // RelPath -> true
}

func NewDirData() *DirData {
	return &DirData{
		FilesByRelPath: make(map[string]string),
		FilesByHash:    make(map[string][]string),
		EmptyDirs:      make(map[string]bool),
	}
}

func hashFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := md5.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func scanDir(dirPath string) (*DirData, error) {
	data := NewDirData()

	err := filepath.Walk(dirPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if path == dirPath {
			return nil
		}

		relPath, err := filepath.Rel(dirPath, path)
		if err != nil {
			return err
		}
		
		// Normalize separators for cross-platform comparison
		relPath = filepath.ToSlash(relPath)

		if info.IsDir() {
			f, err := os.Open(path)
			if err != nil {
				return err
			}
			defer f.Close()
			_, err = f.Readdirnames(1)
			if err == io.EOF {
				data.EmptyDirs[relPath] = true
			}
			return nil
		}

		// Regular file
		hash, err := hashFile(path)
		if err != nil {
			return err
		}

		data.FilesByRelPath[relPath] = hash
		data.FilesByHash[hash] = append(data.FilesByHash[hash], relPath)

		return nil
	})

	return data, err
}

func CompareDirs(dir1, dir2 string, out io.Writer) error {
	data1, err := scanDir(dir1)
	if err != nil {
		return fmt.Errorf("error scanning dir1: %w", err)
	}

	data2, err := scanDir(dir2)
	if err != nil {
		return fmt.Errorf("error scanning dir2: %w", err)
	}

	fmt.Fprintf(out, "Comparing:\n Dir1: %s\n Dir2: %s\n\n", dir1, dir2)

	// 1. Modified files
	var modified []string
	for relPath, hash1 := range data1.FilesByRelPath {
		if hash2, exists := data2.FilesByRelPath[relPath]; exists {
			if hash1 != hash2 {
				modified = append(modified, relPath)
			}
		}
	}
	sort.Strings(modified)
	if len(modified) > 0 {
		fmt.Fprintf(out, "=== Modified Files (Same path, different content) ===\n")
		for _, p := range modified {
			fmt.Fprintf(out, "- %s\n", p)
		}
		fmt.Fprintln(out)
	}

	// 2. Moved/Renamed/Copied Files
	var movedHashes []string
	for hash := range data1.FilesByHash {
		if _, exists := data2.FilesByHash[hash]; exists {
			movedHashes = append(movedHashes, hash)
		}
	}
	sort.Strings(movedHashes)

	hasMoved := false
	for _, hash := range movedHashes {
		paths1 := data1.FilesByHash[hash]
		paths2 := data2.FilesByHash[hash]

		m1 := make(map[string]bool)
		for _, p := range paths1 { m1[p] = true }
		m2 := make(map[string]bool)
		for _, p := range paths2 { m2[p] = true }

		var p1Only, p2Only []string
		for _, p := range paths1 {
			if !m2[p] { p1Only = append(p1Only, p) }
		}
		for _, p := range paths2 {
			if !m1[p] { p2Only = append(p2Only, p) }
		}

		if len(p1Only) > 0 || len(p2Only) > 0 {
			if !hasMoved {
				fmt.Fprintf(out, "=== Moved / Renamed / Copied Files (Same content, different paths) ===\n")
				hasMoved = true
			}
			sort.Strings(paths1)
			sort.Strings(paths2)
			fmt.Fprintf(out, "Content Hash: %s\n", hash[:8])
			fmt.Fprintf(out, "  Dir1 Paths: %s\n", strings.Join(paths1, ", "))
			fmt.Fprintf(out, "  Dir2 Paths: %s\n", strings.Join(paths2, ", "))
			fmt.Fprintln(out)
		}
	}

	// 3. Deleted Files
	var deleted []string
	for relPath, hash := range data1.FilesByRelPath {
		if _, pathExists := data2.FilesByRelPath[relPath]; !pathExists {
			if _, hashExists := data2.FilesByHash[hash]; !hashExists {
				deleted = append(deleted, relPath)
			}
		}
	}
	sort.Strings(deleted)
	if len(deleted) > 0 {
		fmt.Fprintf(out, "=== Deleted Files (Only in Dir1) ===\n")
		for _, p := range deleted {
			fmt.Fprintf(out, "- %s\n", p)
		}
		fmt.Fprintln(out)
	}

	// 4. Added Files
	var added []string
	for relPath, hash := range data2.FilesByRelPath {
		if _, pathExists := data1.FilesByRelPath[relPath]; !pathExists {
			if _, hashExists := data1.FilesByHash[hash]; !hashExists {
				added = append(added, relPath)
			}
		}
	}
	sort.Strings(added)
	if len(added) > 0 {
		fmt.Fprintf(out, "=== Added Files (Only in Dir2) ===\n")
		for _, p := range added {
			fmt.Fprintf(out, "- %s\n", p)
		}
		fmt.Fprintln(out)
	}

	// 5. Empty Directories
	var deletedDirs []string
	for p := range data1.EmptyDirs {
		if !data2.EmptyDirs[p] {
			deletedDirs = append(deletedDirs, p)
		}
	}
	sort.Strings(deletedDirs)
	if len(deletedDirs) > 0 {
		fmt.Fprintf(out, "=== Deleted Empty Directories (Only in Dir1) ===\n")
		for _, p := range deletedDirs {
			fmt.Fprintf(out, "- %s\n", p)
		}
		fmt.Fprintln(out)
	}

	var addedDirs []string
	for p := range data2.EmptyDirs {
		if !data1.EmptyDirs[p] {
			addedDirs = append(addedDirs, p)
		}
	}
	sort.Strings(addedDirs)
	if len(addedDirs) > 0 {
		fmt.Fprintf(out, "=== Added Empty Directories (Only in Dir2) ===\n")
		for _, p := range addedDirs {
			fmt.Fprintf(out, "- %s\n", p)
		}
		fmt.Fprintln(out)
	}

	return nil
}

func runMain() int {
	if len(os.Args) != 4 {
		fmt.Fprintf(os.Stderr, "Usage: %s <dir1> <dir2> <output_file>\n", os.Args[0])
		return 1
	}

	dir1 := os.Args[1]
	dir2 := os.Args[2]
	outFile := os.Args[3]

	out, err := os.Create(outFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating output file: %v\n", err)
		return 1
	}
	defer out.Close()

	if err := CompareDirs(dir1, dir2, out); err != nil {
		fmt.Fprintf(os.Stderr, "Error comparing directories: %v\n", err)
		return 1
	}

	fmt.Printf("Comparison complete. Results written to %s\n", outFile)
	return 0
}

func main() {
	os.Exit(runMain())
}
