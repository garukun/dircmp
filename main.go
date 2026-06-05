package main

import (
	"crypto/md5"
	"encoding/hex"
	"flag"
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

func scanDir(dirPath string, ignoreDSStore bool) (*DirData, error) {
	data := NewDirData()

	err := filepath.Walk(dirPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if path == dirPath {
			return nil
		}

		if ignoreDSStore && filepath.Base(path) == ".DS_Store" {
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
			names, err := f.Readdirnames(-1)
			if err != nil && err != io.EOF {
				return err
			}
			isEmpty := true
			for _, name := range names {
				if !ignoreDSStore || name != ".DS_Store" {
					isEmpty = false
					break
				}
			}
			if isEmpty {
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

func canonicalDirPath(dir string) (string, error) {
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return "", err
	}
	return filepath.EvalSymlinks(absDir)
}

func sameDirectory(dir1, dir2 string) bool {
	path1, err1 := canonicalDirPath(dir1)
	path2, err2 := canonicalDirPath(dir2)
	if err1 != nil || err2 != nil {
		return filepath.Clean(dir1) == filepath.Clean(dir2)
	}
	return path1 == path2
}

func CompareDirs(dir1, dir2 string, ignoreDSStore bool, out io.Writer) error {
	type scanResult struct {
		data *DirData
		err  error
	}

	var data1, data2 *DirData

	if sameDirectory(dir1, dir2) {
		fmt.Printf("Scanning the same directory once.\n")
		d, err := scanDir(dir1, ignoreDSStore)
		if err != nil {
			return fmt.Errorf("error scanning dir1: %w", err)
		}
		data1, data2 = d, d
	} else if shouldScanSequentially(dir1, dir2) {
		// Same mechanical HDD – read sequentially to avoid disk thrashing.
		fmt.Printf("Scanning directories sequentially.\n")
		var err error
		data1, err = scanDir(dir1, ignoreDSStore)
		if err != nil {
			return fmt.Errorf("error scanning dir1: %w", err)
		}
		data2, err = scanDir(dir2, ignoreDSStore)
		if err != nil {
			return fmt.Errorf("error scanning dir2: %w", err)
		}
	} else {
		// Different drives or SSD – read concurrently for maximum throughput.
		fmt.Printf("Scanning directories concurrently.\n")
		ch1 := make(chan scanResult, 1)
		ch2 := make(chan scanResult, 1)

		go func() {
			d, e := scanDir(dir1, ignoreDSStore)
			ch1 <- scanResult{d, e}
		}()
		go func() {
			d, e := scanDir(dir2, ignoreDSStore)
			ch2 <- scanResult{d, e}
		}()

		res1 := <-ch1
		res2 := <-ch2

		if res1.err != nil {
			return fmt.Errorf("error scanning dir1: %w", res1.err)
		}
		if res2.err != nil {
			return fmt.Errorf("error scanning dir2: %w", res2.err)
		}
		data1, data2 = res1.data, res2.data
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
		for _, p := range paths1 {
			m1[p] = true
		}
		m2 := make(map[string]bool)
		for _, p := range paths2 {
			m2[p] = true
		}

		var p1Only, p2Only []string
		for _, p := range paths1 {
			if !m2[p] {
				p1Only = append(p1Only, p)
			}
		}
		for _, p := range paths2 {
			if !m1[p] {
				p2Only = append(p2Only, p)
			}
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
	ignoreDSStore := flag.Bool("ignore-ds-store", false, "Ignore .DS_Store files during comparison")
	flag.Parse()

	args := flag.Args()
	if len(args) != 3 {
		fmt.Fprintf(os.Stderr, "Usage: %s [flags] <dir1> <dir2> <output_file>\n", os.Args[0])
		flag.PrintDefaults()
		return 1
	}

	dir1 := args[0]
	dir2 := args[1]
	outFile := args[2]

	out, err := os.Create(outFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating output file: %v\n", err)
		return 1
	}
	defer out.Close()

	if err := CompareDirs(dir1, dir2, *ignoreDSStore, out); err != nil {
		fmt.Fprintf(os.Stderr, "Error comparing directories: %v\n", err)
		return 1
	}

	fmt.Printf("Comparison complete. Results written to %s\n", outFile)
	return 0
}

func main() {
	os.Exit(runMain())
}
