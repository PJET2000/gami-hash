package engine

import (
	"bytes"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"os"
)

var utf8BOM = []byte{0xEF, 0xBB, 0xBF}

// repairTruncatedTail truncates an existing manifest CSV to the end of its
// last complete, valid record. A crash or power loss can leave a partially
// written final line; cutting it is safe because the affected file is simply
// re-hashed on resume.
func repairTruncatedTail(path string) error {
	f, err := os.OpenFile(path, os.O_RDWR, 0)
	if err != nil {
		return err
	}
	defer f.Close()

	// Skip the BOM if present.
	var start int64
	head := make([]byte, 3)
	if n, _ := io.ReadFull(f, head); n == 3 && bytes.Equal(head, utf8BOM) {
		start = 3
	}
	if _, err := f.Seek(start, io.SeekStart); err != nil {
		return err
	}

	r := csv.NewReader(f)
	r.FieldsPerRecord = len(csvHeader)
	lastGood := start
	first := true
	for {
		rec, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			// Anything from here on is a damaged tail: cut it off.
			break
		}
		if first {
			first = false
			if !sliceEqual(rec, csvHeader) {
				return fmt.Errorf("existing file has an unexpected format (not a manifest written by this tool)")
			}
		}
		lastGood = start + r.InputOffset()
	}
	if first {
		return fmt.Errorf("existing file has no valid header")
	}
	return f.Truncate(lastGood)
}

// readCompletedRels returns the relative paths of all valid rows in a
// manifest CSV (after repairTruncatedTail has run).
func readCompletedRels(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	r := csv.NewReader(skipBOM(f))
	r.FieldsPerRecord = len(csvHeader)
	r.ReuseRecord = true

	var rels []string
	first := true
	for {
		rec, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		if first {
			first = false
			if !sliceEqual(rec, csvHeader) {
				return nil, errors.New("unexpected CSV header")
			}
			continue
		}
		rels = append(rels, rec[0])
	}
	return rels, nil
}

// countValidRows counts complete data rows in a manifest CSV, tolerating a
// damaged tail (used only for the resume prompt, read-only).
func countValidRows(path string) (int64, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer f.Close()
	r := csv.NewReader(skipBOM(f))
	r.FieldsPerRecord = len(csvHeader)
	r.ReuseRecord = true

	var n int64
	first := true
	for {
		rec, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			break // damaged tail: count what is valid
		}
		if first {
			first = false
			if !sliceEqual(rec, csvHeader) {
				return 0, errors.New("unexpected CSV header")
			}
			continue
		}
		n++
	}
	if first {
		return 0, errors.New("no valid header")
	}
	return n, nil
}

func skipBOM(f *os.File) io.Reader {
	head := make([]byte, 3)
	n, _ := io.ReadFull(f, head)
	if n == 3 && bytes.Equal(head, utf8BOM) {
		return f
	}
	f.Seek(0, io.SeekStart)
	return f
}

func sliceEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
