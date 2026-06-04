package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func createTestEnv(t *testing.T) (string, string) {
	dir1 := t.TempDir()
	dir2 := t.TempDir()
	return dir1, dir2
}

func writeFile(t *testing.T, dir, path, content string) {
	fullPath := filepath.Join(dir, path)
	err := os.MkdirAll(filepath.Dir(fullPath), 0755)
	if err != nil {
		t.Fatalf("Failed to create dirs for %s: %v", path, err)
	}
	err = os.WriteFile(fullPath, []byte(content), 0644)
	if err != nil {
		t.Fatalf("Failed to write file %s: %v", path, err)
	}
}

func createEmptyDir(t *testing.T, dir, path string) {
	fullPath := filepath.Join(dir, path)
	err := os.MkdirAll(fullPath, 0755)
	if err != nil {
		t.Fatalf("Failed to create empty dir %s: %v", path, err)
	}
}

func TestCompareDirs_Identical(t *testing.T) {
	dir1, dir2 := createTestEnv(t)

	writeFile(t, dir1, "file1.txt", "content1")
	writeFile(t, dir2, "file1.txt", "content1")
	createEmptyDir(t, dir1, "empty1")
	createEmptyDir(t, dir2, "empty1")

	var buf bytes.Buffer
	err := CompareDirs(dir1, dir2, &buf)
	if err != nil {
		t.Fatalf("CompareDirs failed: %v", err)
	}

	out := buf.String()
	if strings.Contains(out, "=== Modified") || strings.Contains(out, "=== Deleted") || strings.Contains(out, "=== Added") || strings.Contains(out, "=== Moved") {
		t.Errorf("Expected no differences, got:\n%s", out)
	}
}

func TestCompareDirs_Differences(t *testing.T) {
	dir1, dir2 := createTestEnv(t)

	// Modified file
	writeFile(t, dir1, "modified.txt", "v1")
	writeFile(t, dir2, "modified.txt", "v2")

	// Moved/Renamed file
	writeFile(t, dir1, "rename_src.txt", "renamed_content")
	writeFile(t, dir2, "rename_dst.txt", "renamed_content")

	// Deleted file
	writeFile(t, dir1, "deleted.txt", "only_in_1")

	// Added file
	writeFile(t, dir2, "added.txt", "only_in_2")

	// Empty dirs
	createEmptyDir(t, dir1, "del_dir")
	createEmptyDir(t, dir2, "add_dir")

	var buf bytes.Buffer
	err := CompareDirs(dir1, dir2, &buf)
	if err != nil {
		t.Fatalf("CompareDirs failed: %v", err)
	}

	out := buf.String()

	if !strings.Contains(out, "=== Modified Files (Same path, different content) ===") || !strings.Contains(out, "modified.txt") {
		t.Errorf("Expected modified file not found in output:\n%s", out)
	}
	if !strings.Contains(out, "=== Moved / Renamed / Copied Files (Same content, different paths) ===") || !strings.Contains(out, "rename_src.txt") || !strings.Contains(out, "rename_dst.txt") {
		t.Errorf("Expected moved/renamed file not found in output:\n%s", out)
	}
	if !strings.Contains(out, "=== Deleted Files (Only in Dir1) ===") || !strings.Contains(out, "deleted.txt") {
		t.Errorf("Expected deleted file not found in output:\n%s", out)
	}
	if !strings.Contains(out, "=== Added Files (Only in Dir2) ===") || !strings.Contains(out, "added.txt") {
		t.Errorf("Expected added file not found in output:\n%s", out)
	}
	if !strings.Contains(out, "=== Deleted Empty Directories (Only in Dir1) ===") || !strings.Contains(out, "del_dir") {
		t.Errorf("Expected deleted empty dir not found in output:\n%s", out)
	}
	if !strings.Contains(out, "=== Added Empty Directories (Only in Dir2) ===") || !strings.Contains(out, "add_dir") {
		t.Errorf("Expected added empty dir not found in output:\n%s", out)
	}
}

func TestCompareDirs_CopiedFile(t *testing.T) {
	dir1, dir2 := createTestEnv(t)

	writeFile(t, dir1, "src.txt", "copy_content")
	writeFile(t, dir2, "src.txt", "copy_content")
	writeFile(t, dir2, "dst.txt", "copy_content")

	var buf bytes.Buffer
	err := CompareDirs(dir1, dir2, &buf)
	if err != nil {
		t.Fatalf("CompareDirs failed: %v", err)
	}

	out := buf.String()

	if !strings.Contains(out, "=== Moved / Renamed / Copied Files (Same content, different paths) ===") || !strings.Contains(out, "src.txt") || !strings.Contains(out, "dst.txt") {
		t.Errorf("Expected copied file to be detected in output:\n%s", out)
	}
}

func TestCompareDirs_TestData1(t *testing.T) {
	dir1 := filepath.Join("testdata", "test1", "dirA")
	dir2 := filepath.Join("testdata", "test1", "dirB")

	var buf bytes.Buffer
	err := CompareDirs(dir1, dir2, &buf)
	if err != nil {
		t.Fatalf("CompareDirs failed: %v", err)
	}

	out := buf.String()
	t.Logf("Output:\n%s", out)

	// Case 1: Testing for files where the names are the same but the contents are different (Modified Files)
	// target: sub2/same_name_only.txt
	if !strings.Contains(out, "=== Modified Files (Same path, different content) ===") {
		t.Error("Expected modified files section to be present")
	}
	if !strings.Contains(out, "- sub2/same_name_only.txt") {
		t.Error("Expected 'sub2/same_name_only.txt' to be detected as modified")
	}

	// Case 2: Testing for files where the names are different but the contents are the same (Moved / Renamed / Copied)
	// target: sub2/diff_name_only_a.txt in dirA (content matches sub2/diff_name_only_b.txt in dirB)
	if !strings.Contains(out, "=== Moved / Renamed / Copied Files (Same content, different paths) ===") {
		t.Error("Expected moved/renamed/copied section to be present")
	}
	if !strings.Contains(out, "sub2/diff_name_only_a.txt") || !strings.Contains(out, "sub2/diff_name_only_b.txt") {
		t.Error("Expected different-named but identical-content files to be grouped under moved/renamed/copied")
	}

	// Case 3: Testing for files where the contents and file names are the same but they are in different paths
	// target: sub2/diff_path_a.txt in dirA (content and name matches sub1/diff_path_a.txt in dirB)
	if !strings.Contains(out, "sub2/diff_path_a.txt") || !strings.Contains(out, "sub1/diff_path_a.txt") {
		t.Error("Expected same name/content but different path files to be grouped under moved/renamed/copied")
	}

	// Case 4: Testing for files that exist in one directory but not in the other (Deleted / Added Files)
	// target: sub2/additional_file.txt exists only in dirA, sub1/additional_file.txt exists only in dirB
	if !strings.Contains(out, "=== Deleted Files (Only in Dir1) ===") {
		t.Error("Expected deleted files section to be present")
	}
	if !strings.Contains(out, "- sub2/additional_file.txt") {
		t.Error("Expected 'sub2/additional_file.txt' to be detected as deleted")
	}
	if !strings.Contains(out, "=== Added Files (Only in Dir2) ===") {
		t.Error("Expected added files section to be present")
	}
	if !strings.Contains(out, "- sub1/additional_file.txt") {
		t.Error("Expected 'sub1/additional_file.txt' to be detected as added")
	}

	// Case 5: Testing duplicate content in three different locations (present in both directories identically)
	// target: sub1/duplicate_in_three.txt, sub2/duplicate_in_three.txt, sub3/duplicate_in_three.txt
	// These are identical across both directories, so they should NOT show up as differences.
	if strings.Contains(out, "duplicate_in_three.txt") {
		t.Error("Files with identical content and path in 3 locations should not be reported as differences")
	}

	// Case 6: Happy-path test cases in subdirectories and recursive subdirectories that are identical
	// target: sub2/sub2a/f1.txt, sub1/f3.txt, sub2/f2.txt
	if strings.Contains(out, "sub2/sub2a/f1.txt") || strings.Contains(out, "sub1/f3.txt") || strings.Contains(out, "sub2/f2.txt") {
		t.Error("Identical happy-path files in subdirectories should not be reported as differences")
	}

	// Case 7: Testing for empty directories that are identical
	// target: emptyDir1 is present in both and should not be reported as deleted or added.
	if strings.Contains(out, "emptyDir1") {
		t.Error("Identical empty directories should not be reported as differences")
	}

	// Case 8: Testing for empty directories that uniquely exist in either input directory
	// target: additionalEmptyDirA (deleted), additionalEmptyDirB (added)
	if !strings.Contains(out, "=== Deleted Empty Directories (Only in Dir1) ===") || !strings.Contains(out, "- additionalEmptyDirA") {
		t.Error("Expected 'additionalEmptyDirA' to be detected as a deleted empty directory")
	}
	if !strings.Contains(out, "=== Added Empty Directories (Only in Dir2) ===") || !strings.Contains(out, "- additionalEmptyDirB") {
		t.Error("Expected 'additionalEmptyDirB' to be detected as an added empty directory")
	}
}
