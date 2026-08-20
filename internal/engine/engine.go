// Package engine implements the core hashing run: it walks a folder tree,
// computes SHA-256 for every regular file and writes one CSV manifest.
//
// Guarantees:
//   - Strictly read-only towards the scanned tree (files are opened O_RDONLY,
//     nothing is ever created or modified under the root).
//   - Per-file errors (permissions, vanished files, I/O errors) never abort
//     the run; they are appended to an error log and the file is skipped.
//   - Runs are resumable: the CSV itself is the record of completed files, a
//     small sidecar checkpoint file marks an unfinished run and remembers the
//     root folder. On resume, files already present in the CSV are skipped.
//   - No network access anywhere (the whole program has no networking code).
package engine

import (
	"context"
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unicode/utf8"
)

// Version of the tool. Overridden at build time via
// -ldflags "-X .../internal/engine.Version=v1.0.0".
var Version = "dev"

// CSV layout. The header is fixed; a resume run refuses to append to a file
// with a different header.
var csvHeader = []string{"relative_path", "filename", "size_bytes", "sha256", "mtime_utc"}

const (
	checkpointSuffix = ".part.json"
	errorLogSuffix   = "_errors.log"
	readBufSize      = 1 << 20 // 1 MiB
	flushEveryRows   = 64
	flushEvery       = 2 * time.Second
)

// DefaultWorkers is the default number of parallel hashing workers. It is
// deliberately low: on spinning disks (the common case for archive drives)
// parallel reads cause seeking and are often slower than sequential reads.
const DefaultWorkers = 2

// Options configures a run.
type Options struct {
	Root    string // folder to scan (must exist)
	Output  string // CSV file to write (must not be inside Root)
	Workers int    // parallel hashing workers; <1 means DefaultWorkers
	Fresh   bool   // true: ignore any resumable state and start over

	// AllowRootMismatch resumes even when the checkpoint records a different
	// root path. Needed when the same external drive reappears under a new
	// drive letter or mount point: relative paths still match, so the resume
	// is correct. Only set when the user confirmed it is the same folder.
	AllowRootMismatch bool
}

// Phase of a run, reported through Progress.
type Phase int

const (
	PhaseScan Phase = iota // counting files (fast pre-pass)
	PhaseHash              // hashing
)

// Progress is a point-in-time snapshot passed to the progress callback.
type Progress struct {
	Phase      Phase
	FilesDone  int64 // hashed + resumed + failed
	FilesTotal int64
	BytesDone  int64
	BytesTotal int64
}

// Result summarises a finished (or cancelled) run.
type Result struct {
	FilesHashed  int64 // hashed during this run
	FilesResumed int64 // skipped because a previous run already hashed them
	FilesFailed  int64 // unreadable, vanished, ... (see error log)
	Skipped      int64 // non-regular entries (symlinks, sockets, ...)
	BytesHashed  int64
	FilesTotal   int64
	BytesTotal   int64
	Output       string
	ErrorLog     string // "" if no error log was written
	Canceled     bool
	Elapsed      time.Duration
}

// ResumeState describes whether an interrupted run exists for an output file.
type ResumeState struct {
	Resumable bool   // checkpoint + CSV exist and are consistent
	Root      string // root folder recorded in the checkpoint
	RowCount  int64  // completed rows in the existing CSV
}

type checkpoint struct {
	Version    string `json:"tool_version"`
	Root       string `json:"root"`
	Started    string `json:"started_utc"`
	CSVHeader  string `json:"csv_header"`
	FormatNote string `json:"note"`
}

// CheckpointPath returns the sidecar checkpoint path for an output CSV.
func CheckpointPath(output string) string { return output + checkpointSuffix }

// ErrorLogPath returns the error log path for an output CSV
// (foo.csv -> foo_errors.log).
func ErrorLogPath(output string) string {
	base := strings.TrimSuffix(output, filepath.Ext(output))
	return base + errorLogSuffix
}

// CheckResume inspects output and reports whether an interrupted run can be
// resumed. It never modifies anything.
func CheckResume(output string) ResumeState {
	var st ResumeState
	data, err := os.ReadFile(CheckpointPath(output))
	if err != nil {
		return st
	}
	var cp checkpoint
	if json.Unmarshal(data, &cp) != nil || cp.Root == "" {
		return st
	}
	if _, err := os.Stat(output); err != nil {
		return st
	}
	rows, err := countValidRows(output)
	if err != nil {
		return st
	}
	st.Resumable = true
	st.Root = cp.Root
	st.RowCount = rows
	return st
}

// SameRoot reports whether two cleaned absolute paths refer to the same root,
// using case-insensitive comparison on Windows.
func SameRoot(a, b string) bool {
	a, b = filepath.Clean(a), filepath.Clean(b)
	if runtime.GOOS == "windows" {
		return strings.EqualFold(a, b)
	}
	return a == b
}

// isWithin reports whether path is inside (or equal to) dir. Both must be
// absolute; comparison is lexical, case-insensitive on Windows.
func isWithin(path, dir string) bool {
	path, dir = filepath.Clean(path), filepath.Clean(dir)
	if runtime.GOOS == "windows" {
		path, dir = strings.ToLower(path), strings.ToLower(dir)
	}
	if path == dir {
		return true
	}
	return strings.HasPrefix(path, dir+string(filepath.Separator))
}

// job is one file to hash, in deterministic walk order.
type job struct {
	index int64
	rel   string // slash-separated relative path
	abs   string
	size  int64
	mtime time.Time
}

// outcome is the result of hashing one file.
type outcome struct {
	index int64
	rel   string
	name  string
	size  int64
	mtime time.Time
	hash  string
	err   error // non-nil: file failed, goes to the error log
}

type engineRun struct {
	opts     Options
	ctx      context.Context
	progress func(Progress)

	skip map[string]struct{} // relpaths already in the CSV (resume)

	filesTotal  atomic.Int64
	bytesTotal  atomic.Int64
	filesDone   atomic.Int64
	bytesDone   atomic.Int64
	filesFailed atomic.Int64 // written by producer and writer goroutines
	skipped     atomic.Int64

	res Result

	errLogMu sync.Mutex
	errLog   *os.File
}

// Run executes a hashing run. progressFn may be nil; it is called from a
// single goroutine, throttled to roughly one call per 100ms.
//
// Fatal setup problems (bad root, output inside root, unwritable output)
// return an error. Per-file problems never do.
func Run(ctx context.Context, opts Options, progressFn func(Progress)) (Result, error) {
	start := time.Now()
	if opts.Workers < 1 {
		opts.Workers = DefaultWorkers
	}
	if progressFn == nil {
		progressFn = func(Progress) {}
	}

	var err error
	opts.Root, err = filepath.Abs(opts.Root)
	if err != nil {
		return Result{}, fmt.Errorf("invalid root folder: %w", err)
	}
	opts.Root = filepath.Clean(opts.Root)
	opts.Output, err = filepath.Abs(opts.Output)
	if err != nil {
		return Result{}, fmt.Errorf("invalid output path: %w", err)
	}
	opts.Output = filepath.Clean(opts.Output)

	info, err := os.Stat(opts.Root)
	if err != nil {
		return Result{}, fmt.Errorf("cannot access root folder: %w", err)
	}
	if !info.IsDir() {
		return Result{}, fmt.Errorf("root is not a folder: %s", opts.Root)
	}
	// The one hard safety rule: we never write into the scanned tree.
	if isWithin(opts.Output, opts.Root) {
		return Result{}, fmt.Errorf("output file must not be inside the scanned folder")
	}
	if dir := filepath.Dir(opts.Output); true {
		if di, derr := os.Stat(dir); derr != nil || !di.IsDir() {
			return Result{}, fmt.Errorf("output folder does not exist: %s", dir)
		}
	}

	r := &engineRun{opts: opts, ctx: ctx, progress: progressFn, skip: map[string]struct{}{}}
	r.res.Output = opts.Output

	// ---- Resume handling -------------------------------------------------
	resuming := false
	if !opts.Fresh {
		st := CheckResume(opts.Output)
		if st.Resumable && (SameRoot(st.Root, opts.Root) || opts.AllowRootMismatch) {
			resuming = true
		}
	}

	var csvFile *os.File
	if resuming {
		if err := repairTruncatedTail(opts.Output); err != nil {
			return Result{}, fmt.Errorf("cannot repair existing CSV: %w", err)
		}
		rels, err := readCompletedRels(opts.Output)
		if err != nil {
			return Result{}, fmt.Errorf("cannot read existing CSV: %w", err)
		}
		for _, rel := range rels {
			r.skip[rel] = struct{}{}
		}
		csvFile, err = os.OpenFile(opts.Output, os.O_WRONLY|os.O_APPEND, 0o644)
		if err != nil {
			return Result{}, fmt.Errorf("cannot open output file: %w", err)
		}
	} else {
		csvFile, err = os.Create(opts.Output)
		if err != nil {
			return Result{}, fmt.Errorf("cannot create output file: %w", err)
		}
		// UTF-8 BOM so Excel opens umlauts correctly, then the header.
		if _, err := csvFile.Write([]byte{0xEF, 0xBB, 0xBF}); err != nil {
			csvFile.Close()
			return Result{}, fmt.Errorf("cannot write output file: %w", err)
		}
		w := csv.NewWriter(csvFile)
		w.UseCRLF = true
		if err := w.Write(csvHeader); err == nil {
			w.Flush()
		}
		if err := w.Error(); err != nil {
			csvFile.Close()
			return Result{}, fmt.Errorf("cannot write output file: %w", err)
		}
	}
	defer csvFile.Close()

	// Checkpoint marks "run in progress". Removed only on full success.
	cp := checkpoint{
		Version:    Version,
		Root:       opts.Root,
		Started:    time.Now().UTC().Format(time.RFC3339),
		CSVHeader:  strings.Join(csvHeader, ","),
		FormatNote: "temporary file; marks an interrupted hashing run so it can be resumed. Safe to delete.",
	}
	// Always (re)written so the recorded root stays current — e.g. after an
	// external drive came back under a different drive letter.
	cpData, _ := json.MarshalIndent(cp, "", "  ")
	if err := os.WriteFile(CheckpointPath(opts.Output), cpData, 0o644); err != nil {
		return Result{}, fmt.Errorf("cannot write checkpoint file: %w", err)
	}

	defer func() {
		if r.errLog != nil {
			r.errLog.Close()
		}
	}()

	// ---- Pass 1: count files and bytes (for the progress display) --------
	r.progress(Progress{Phase: PhaseScan})
	scanTick := time.Now()
	err = r.walk(func(rel, abs string, info fs.FileInfo) {
		r.filesTotal.Add(1)
		r.bytesTotal.Add(info.Size())
		if time.Since(scanTick) > 100*time.Millisecond {
			scanTick = time.Now()
			r.progress(Progress{Phase: PhaseScan, FilesTotal: r.filesTotal.Load(), BytesTotal: r.bytesTotal.Load()})
		}
	}, false)
	if err != nil {
		return Result{}, err
	}
	if r.ctx.Err() != nil {
		r.res.Canceled = true
		r.res.Elapsed = time.Since(start)
		return r.res, nil
	}
	r.res.FilesTotal = r.filesTotal.Load()
	r.res.BytesTotal = r.bytesTotal.Load()

	// ---- Pass 2: hash ------------------------------------------------------
	jobs := make(chan job, 4*opts.Workers)
	results := make(chan outcome, 4*opts.Workers)

	var wg sync.WaitGroup
	for i := 0; i < opts.Workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			r.worker(jobs, results)
		}()
	}

	// Progress reporter.
	repCtx, repCancel := context.WithCancel(context.Background())
	var repWG sync.WaitGroup
	repWG.Add(1)
	go func() {
		defer repWG.Done()
		t := time.NewTicker(100 * time.Millisecond)
		defer t.Stop()
		for {
			select {
			case <-repCtx.Done():
				return
			case <-t.C:
				r.progress(r.snapshot())
			}
		}
	}()

	// Writer goroutine: emits rows in deterministic walk order.
	var writeErr error
	var writerWG sync.WaitGroup
	writerWG.Add(1)
	go func() {
		defer writerWG.Done()
		writeErr = r.writeLoop(csvFile, results)
	}()

	// Producer: walk again with identical order, enqueue jobs.
	var index int64
	prodErr := r.walk(func(rel, abs string, info fs.FileInfo) {
		if _, done := r.skip[rel]; done {
			r.res.FilesResumed++
			r.filesDone.Add(1)
			r.bytesDone.Add(info.Size())
			return
		}
		select {
		case jobs <- job{index: index, rel: rel, abs: abs, size: info.Size(), mtime: info.ModTime()}:
			index++
		case <-r.ctx.Done():
		}
	}, true)
	close(jobs)
	wg.Wait()
	close(results)
	writerWG.Wait()
	repCancel()
	repWG.Wait()
	r.res.FilesFailed = r.filesFailed.Load()
	r.res.Skipped = r.skipped.Load()

	if prodErr != nil {
		return Result{}, prodErr
	}
	if writeErr != nil {
		return Result{}, fmt.Errorf("cannot write output file: %w", writeErr)
	}

	if err := csvFile.Sync(); err != nil {
		return Result{}, fmt.Errorf("cannot flush output file: %w", err)
	}

	r.res.Elapsed = time.Since(start)
	if r.ctx.Err() != nil {
		r.res.Canceled = true
		r.progress(r.snapshot())
		return r.res, nil
	}

	// Success: remove the checkpoint marker.
	if err := os.Remove(CheckpointPath(opts.Output)); err != nil && !errors.Is(err, fs.ErrNotExist) {
		r.logError("", fmt.Errorf("could not remove checkpoint file: %w", err))
	}
	r.progress(Progress{
		Phase: PhaseHash, FilesDone: r.res.FilesTotal, FilesTotal: r.res.FilesTotal,
		BytesDone: r.res.BytesTotal, BytesTotal: r.res.BytesTotal,
	})
	return r.res, nil
}

func (r *engineRun) snapshot() Progress {
	return Progress{
		Phase:      PhaseHash,
		FilesDone:  r.filesDone.Load(),
		FilesTotal: r.filesTotal.Load(),
		BytesDone:  r.bytesDone.Load(),
		BytesTotal: r.bytesTotal.Load(),
	}
}

// walk traverses the tree depth-first with entries sorted byte-wise, so both
// passes and any two runs see files in the same order. fn is called for
// regular files only. When logIssues is true, unreadable directories and
// skipped non-regular entries are recorded in the error log.
func (r *engineRun) walk(fn func(rel, abs string, info fs.FileInfo), logIssues bool) error {
	// Our own files must never end up in the manifest, even if the user put
	// the output right next to (or, blocked elsewhere, inside) the tree.
	excluded := []string{r.opts.Output, CheckpointPath(r.opts.Output), ErrorLogPath(r.opts.Output)}
	isExcluded := func(abs string) bool {
		for _, p := range excluded {
			if SameRoot(abs, p) {
				return true
			}
		}
		return false
	}

	var visit func(abs, rel string) error
	visit = func(abs, rel string) error {
		if r.ctx.Err() != nil {
			return nil
		}
		entries, err := os.ReadDir(abs)
		if err != nil {
			if logIssues {
				r.filesFailed.Add(1) // a whole directory is missing from the manifest
				r.logError(rel, fmt.Errorf("cannot read folder: %w", err))
			}
			return nil
		}
		sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
		for _, e := range entries {
			if r.ctx.Err() != nil {
				return nil
			}
			childAbs := filepath.Join(abs, e.Name())
			// Old drives sometimes carry filenames in legacy encodings
			// (cp1252 etc.) or with control characters. The manifest must
			// stay valid single-line UTF-8, so such bytes are recorded
			// percent-encoded (lossless and reversible); the original raw
			// name is kept in the error log.
			name := encodeNameSpecials(e.Name())
			childRel := name
			if rel != "" {
				childRel = rel + "/" + name
			}
			if name != e.Name() && logIssues {
				r.logError(childRel, fmt.Errorf(
					"filename contains control characters or bytes in a legacy encoding; recorded percent-encoded, raw name: %s", strconv.Quote(e.Name())))
			}
			if isExcluded(childAbs) {
				continue
			}
			switch {
			case e.IsDir():
				if err := visit(childAbs, childRel); err != nil {
					return err
				}
			case !e.Type().IsRegular():
				// Symlinks, sockets, devices: never followed (a link could
				// point anywhere, including outside the tree or in a cycle).
				if logIssues {
					r.skipped.Add(1)
					r.logError(childRel, fmt.Errorf("skipped: not a regular file (%s)", entryTypeName(e.Type())))
				}
			default:
				info, err := e.Info()
				if err != nil {
					if logIssues {
						r.filesFailed.Add(1)
						r.filesDone.Add(1)
						r.logError(childRel, fmt.Errorf("cannot read file attributes: %w", err))
					}
					continue
				}
				fn(childRel, childAbs, info)
			}
		}
		return nil
	}
	return visit(r.opts.Root, "")
}

func entryTypeName(m fs.FileMode) string {
	switch {
	case m&fs.ModeSymlink != 0:
		return "symlink"
	case m&fs.ModeDevice != 0:
		return "device"
	case m&fs.ModeNamedPipe != 0:
		return "named pipe"
	case m&fs.ModeSocket != 0:
		return "socket"
	default:
		return m.Type().String()
	}
}

// worker hashes files from jobs and sends outcomes to results.
func (r *engineRun) worker(jobs <-chan job, results chan<- outcome) {
	buf := make([]byte, readBufSize)
	for j := range jobs {
		if r.ctx.Err() != nil {
			// Drain quickly on cancel; unprocessed files stay unhashed and
			// will be picked up on resume.
			continue
		}
		out := outcome{index: j.index, rel: j.rel, name: relBase(j.rel), size: j.size, mtime: j.mtime}
		hashed, err := r.hashFile(j.abs, buf)
		if err != nil {
			out.err = err
			// Keep the progress bar honest: count the unread remainder.
			r.bytesDone.Add(j.size - hashed.bytes)
		} else {
			out.hash = hashed.sum
			out.size = hashed.bytes // actual bytes hashed (file may have grown/shrunk)
		}
		r.filesDone.Add(1)
		select {
		case results <- out:
		case <-r.ctx.Done():
			// Writer may already be gone; do not block.
			select {
			case results <- out:
			default:
			}
		}
	}
}

type hashed struct {
	sum   string
	bytes int64
}

// hashFile opens a file strictly read-only and streams it through SHA-256.
func (r *engineRun) hashFile(abs string, buf []byte) (hashed, error) {
	f, err := os.Open(abs) // O_RDONLY
	if err != nil {
		return hashed{}, err
	}
	defer f.Close()
	h := sha256.New()
	var n int64
	for {
		if r.ctx.Err() != nil {
			return hashed{bytes: n}, r.ctx.Err()
		}
		read, rerr := f.Read(buf)
		if read > 0 {
			h.Write(buf[:read])
			n += int64(read)
			r.bytesDone.Add(int64(read))
		}
		if rerr == io.EOF {
			break
		}
		if rerr != nil {
			return hashed{bytes: n}, rerr
		}
	}
	return hashed{sum: hex.EncodeToString(h.Sum(nil)), bytes: n}, nil
}

// writeLoop receives outcomes and writes CSV rows in walk order (buffering
// out-of-order results), flushing regularly so a crash loses little work.
func (r *engineRun) writeLoop(f *os.File, results <-chan outcome) error {
	w := csv.NewWriter(f)
	w.UseCRLF = true
	pending := map[int64]outcome{}
	var next int64
	rowsSinceFlush := 0
	lastFlush := time.Now()

	emit := func(o outcome) error {
		if o.err != nil {
			if errors.Is(o.err, context.Canceled) {
				return nil // not a real file error, just shutdown
			}
			r.filesFailed.Add(1)
			r.logError(o.rel, o.err)
			return nil
		}
		r.res.FilesHashed++
		r.res.BytesHashed += o.size
		err := w.Write([]string{
			o.rel,
			o.name,
			strconv.FormatInt(o.size, 10),
			o.hash,
			o.mtime.UTC().Format(time.RFC3339),
		})
		if err != nil {
			return err
		}
		rowsSinceFlush++
		if rowsSinceFlush >= flushEveryRows || time.Since(lastFlush) > flushEvery {
			w.Flush()
			if err := w.Error(); err != nil {
				return err
			}
			rowsSinceFlush = 0
			lastFlush = time.Now()
		}
		return nil
	}

	for o := range results {
		pending[o.index] = o
		for {
			p, ok := pending[next]
			if !ok {
				break
			}
			delete(pending, next)
			next++
			if err := emit(p); err != nil {
				// Output disk failed: drain and report.
				for range results {
				}
				return err
			}
		}
	}
	// On cancel there may be a gap in indices; everything after the gap is
	// dropped and will be re-hashed on resume (correctness over speed).
	w.Flush()
	return w.Error()
}

// logError appends one line to the error log (created on first use).
func (r *engineRun) logError(rel string, err error) {
	r.errLogMu.Lock()
	defer r.errLogMu.Unlock()
	if r.errLog == nil {
		f, ferr := os.OpenFile(ErrorLogPath(r.opts.Output), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
		if ferr != nil {
			return // nowhere to log; never abort the run for this
		}
		r.errLog = f
		r.res.ErrorLog = ErrorLogPath(r.opts.Output)
	}
	line := fmt.Sprintf("%s\t%s\t%s\n",
		time.Now().UTC().Format(time.RFC3339), strconv.Quote(rel), err.Error())
	r.errLog.WriteString(line)
}

func fileExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

// relBase returns the last component of a slash-separated relative path
// (the sanitized filename, matching the relative_path column exactly).
func relBase(rel string) string {
	if i := strings.LastIndexByte(rel, '/'); i >= 0 {
		return rel[i+1:]
	}
	return rel
}

// encodeNameSpecials returns s with every byte that is not part of a valid
// UTF-8 sequence, every control character (U+0000..U+001F) and DEL replaced
// by %XX. Ordinary names — including umlauts, spaces, commas, quotes and
// other Unicode — pass through unchanged.
func encodeNameSpecials(s string) string {
	isControl := func(r rune) bool { return r < 0x20 || r == 0x7F }
	if utf8.ValidString(s) && !strings.ContainsFunc(s, isControl) {
		return s
	}
	var b strings.Builder
	for i := 0; i < len(s); {
		r, size := utf8.DecodeRuneInString(s[i:])
		switch {
		case (r == utf8.RuneError && size == 1) || isControl(r):
			fmt.Fprintf(&b, "%%%02X", s[i])
		default:
			b.WriteString(s[i : i+size])
		}
		i += size
	}
	return b.String()
}
