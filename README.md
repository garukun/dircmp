# Directory Comparison Utility (`dircmp`)

A fast and comprehensive directory comparison tool written in Go. It compares two directories and generates a detailed report of structural and content differences.

## Features

This program recursively scans and indexes target directories to report differences categorized into five distinct areas:

1. **Modified Files**: Identifies files that reside in the same relative path across both directories but contain different content (verified using MD5 hashing).
2. **Moved / Renamed / Copied Files**: Groups files by their MD5 content hashes to detect when a file has been renamed, moved to another path, or duplicated.
3. **Deleted Files**: Identifies files present in the source directory (`dir1`) but completely missing in the destination directory (`dir2`) by both path and content.
4. **Added Files**: Identifies files present in the destination directory (`dir2`) but completely missing in the source directory (`dir1`) by both path and content.
5. **Added/Deleted Empty Directories**: Reports empty subdirectories that exist in one directory but not the other.

### Key Technical Aspects

* **MD5 Hashing**: Performs content validation using MD5 hashing to accurately identify identical contents regardless of filename or directory path.
* **Path Separation Normalization**: Automatically converts all platform-specific path separators to forward slashes (`/`), ensuring consistent comparisons across Windows, macOS, and Linux.
* **Recursive Traversal**: Recursively scans directories to detect modifications, moves, and additions inside nested structures.

## Installation & Requirements

Ensure you have [Go](https://go.dev/) installed (version 1.16 or newer recommended).

```bash
# Clone the repository
git clone git@github.com:garukun/dircmp.git

# Navigate into the project directory
cd dircmp
```

## Usage

Run the program by specifying the two directories to compare and the path to the output report file:

```bash
go run main.go [flags] <dir1> <dir2> <output_report_file>
```

### Flags

* `-ignore-ds-store` (default: `false`): Ignores macOS `.DS_Store` files and considers directories containing only `.DS_Store` files as empty. Set to `true` to exclude `.DS_Store` files from the comparison.

### Example

```bash
# Compare directories including .DS_Store files (default)
go run main.go testdata/test1/dirA testdata/test1/dirB out/report.txt

# Compare directories while ignoring .DS_Store files
go run main.go -ignore-ds-store=true testdata/test1/dirA testdata/test1/dirB out/report.txt
```

## Running Tests

The project includes a comprehensive test suite in `main_test.go` covering edge cases like duplicated content in multiple locations, empty directories, added/deleted files, modifications, and renames.

Run the tests using the following command:

```bash
go test -v ./...
```
