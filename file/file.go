// Package file provides the capability to parse from and write to disk a roster
// configuration and index file.
// The roster file is currently implemented in YAML format to minimize file size
// and also permit user annotation with comments.
package file

import (
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"sync"
	"unicode/utf8"

	"github.com/cespare/xxhash"
	"gopkg.in/yaml.v3"
)

type (
	DirectoryNotFoundError string
	InvalidPathError       string
	NotRegularFileError    string
)

// Error returns the error message for DirectoryNotFoundError.
func (e DirectoryNotFoundError) Error() string {
	return "directory not found: " + string(e)
}

// Error returns the error message for InvalidPathError.
func (e InvalidPathError) Error() string {
	return "invalid file path: " + string(e)
}

// Error returns the error message for NotRegularFileError.
func (e NotRegularFileError) Error() string {
	return "not a regular file: " + string(e)
}

// Permissions defines the default permissions of roster files written to disk.
var Permissions os.FileMode = 0600

// Roster represents a roster file, containing the index of all member files in
// a directory tree.
type Roster struct {
	path  string
	memlk sync.Mutex
	abslk sync.Mutex
	Cfg   Config `yaml:"config"`  // roster configuration
	Mem   Member `yaml:"members"` // index of all files
	abs   Absent
}

// IgnoreDefault defines the default Ignore patterns used when creating a new
// roster file. The default items are VCS metadata directories.
var IgnoreDefault = Ignore{"\\.git", "\\.svn"}

// Config contains settings for constructing and verifying the roster index.
type Config struct {
	Rt  Runtime `yaml:"runtime"` // various runtime settings
	Ver Verify  `yaml:"verify"`  // attributes used to identify changed files
	Ign Ignore  `yaml:"ignore"`  // file patterns to exclude from roster index
	ire IgnoreRegexp
}

// Constants representing special-purpose values for Runtime fields.
const (
	RuntimeThreadsNoLimit = 0 // number of threads limited to number of CPUs
	RuntimeDepthNoLimit   = 0 // unlimited recursion
)

// Runtime fine-tunes the construction/verification operations.
type Runtime struct {
	Thr int `yaml:"threads"`
	Dep int `yaml:"maxdepth"`
}

// AllVerify returns a Verify struct with all attributes set true for
// verification.
func AllVerify() Verify {
	return Verify{Fsize: true, Perms: true, Mtime: true, Check: true}
}

// Verify defines file attributes that are recorded for all indexed files and
// used to identify changed files.
type Verify struct {
	Fsize bool `yaml:"filesize"`
	Perms bool `yaml:"permissions"`
	Mtime bool `yaml:"lastmodtime"`
	Check bool `yaml:"checksum"`
}

// Ignore stores a list of file patterns to exclude from the roster index.
type Ignore []string

// IgnoreRegexp stores a list of compiled regular expressions created from a
// slice of strings of type Ignore.
type IgnoreRegexp []*regexp.Regexp

// Compile builds a list of regular expressions from a string slice of ignore
// patterns.
func (i Ignore) Compile() (*IgnoreRegexp, error) {
	ignre := IgnoreRegexp{}
	for _, ign := range i {
		// test if provided a string literal (surrounded with backticks)
		if utf8.RuneCountInString(ign) >= 2 {
			s, sl := utf8.DecodeRuneInString(ign)
			e, el := utf8.DecodeLastRuneInString(ign)
			if s == '`' && e == '`' {
				b := []byte(ign)[sl : len(ign)-el]
				if !utf8.Valid(b) {
					return nil, fmt.Errorf("invalid ignore pattern: %s", ign)
				}
				re, err := regexp.Compile(regexp.QuoteMeta(string(b)))
				if nil != err {
					return nil, err
				}
				ignre = append(ignre, re)
				continue
			}
		}
		re, err := regexp.Compile(ign)
		if nil != err {
			return nil, err
		}
		ignre = append(ignre, re)
	}
	return &ignre, nil
}

// Member stores the index of all roster members as a mapping from file path to
// Status struct containing file attributes.
type Member map[string]Status

// Absent stores a record of all files in the roster, which are removed once the
// file is discovered.
type Absent map[string]bool

// Constants defining Status field values with special meaning.
const (
	StatusNoFsize   int64  = -1
	StatusPermsMask uint64 = 0x00000000FFFFFFFF
	StatusNoPerms   string = "(none)"
	StatusNoMtime   string = "(none)"
	StatusNoCheck   string = ""
)

// Status represents all verifiable attributes of an indexed file.
type Status struct {
	Fsize int64  `yaml:"size"`
	Perms string `yaml:"perm"`
	Mtime string `yaml:"last"`
	Check string `yaml:"hash"`
}

// NoStatus returns a default Status struct for files that have not been
// analyzed.
func NoStatus() Status {
	return Status{
		Fsize: StatusNoFsize,
		Perms: StatusNoPerms,
		Mtime: StatusNoMtime,
		Check: StatusNoCheck,
	}
}

// MakeStatus constructs a new Status struct. This method does not consider the
// Verify settings, and it will always analyze all attributes of the given file.
func MakeStatus(root string, relPath string, info os.FileInfo) (Status, error) {
	var stat Status

	stat.Fsize = info.Size()
	stat.Perms = info.Mode().String()
	stat.Mtime = info.ModTime().Local().String()

	// compute checksum
	var err error
	if stat.Check, err = Checksum(filepath.Join(root, relPath)); nil != err {
		return NoStatus(), err
	}

	return stat, nil
}

// Valid verifies the receiver Status s is not equal to the unique NoStatus
// struct, using all Status attributes.
func (s Status) Valid() bool {
	return !s.Equals(NoStatus(), AllVerify())
}

// Equals compares two Status structs for equality, per Verify settings.
func (s Status) Equals(t Status, ver Verify) bool {
	return (!ver.Fsize || s.Fsize == t.Fsize) &&
		(!ver.Perms || s.Perms == t.Perms) &&
		(!ver.Mtime || s.Mtime == t.Mtime) &&
		(!ver.Check || s.Check == t.Check)
}

// Checksum computes the checksum of a file at given path.
func Checksum(filePath string) (sum string, err error) {
	f, err := os.Open(filePath)
	if nil != err {
		return "", err
	}
	defer f.Close()

	h := xxhash.New()

	// use io.Copy to stream bytes in file to hashing function
	if _, err := io.Copy(h, f); nil != err {
		return "", err
	}

	// convert resulting hash to hex string
	return strconv.FormatUint(h.Sum64(), 16), nil
}

// New constructs a new roster file at the given file path, initialized with all
// default data.
// The returned file is stored in-memory only. The Write method must be called
// to write the file to disk.
func New(fileExists bool, filePath string) *Roster {
	ign := &Ignore{}
	ire := &IgnoreRegexp{}
	if !fileExists {
		ign = &IgnoreDefault
		ire, _ = ign.Compile()
	}
	return &Roster{
		path:  filePath,
		memlk: sync.Mutex{},
		abslk: sync.Mutex{},
		Cfg: Config{
			Rt: Runtime{
				Thr: RuntimeThreadsNoLimit,
				Dep: RuntimeDepthNoLimit,
			},
			Ver: Verify{
				Fsize: true,
				Perms: false,
				Mtime: false,
				Check: true,
			},
			Ign: *ign,
			ire: *ire,
		},
		Mem: Member{},
		abs: Absent{},
	}
}

// Parse parses the roster configuration and member data from a given roster
// file into the returned Roster struct, or returns a Roster struct with default
// configuration and empty member data if the roster file does not exist.
// Returns a nil Roster and descriptive error if the given path is invalid.
func Parse(filePath string) (*Roster, error) {

	dir := filepath.Dir(filePath)
	dstat, derr := os.Stat(dir)
	if os.IsNotExist(derr) {
		return nil, DirectoryNotFoundError(dir)
	} else if !dstat.IsDir() {
		return nil, InvalidPathError(dir)
	}

	fstat, ferr := os.Stat(filePath)
	if os.IsNotExist(ferr) {
		// create a new default roster file if one does not exist
		return New(false, filePath), nil
	} else if uint32(fstat.Mode()&os.ModeType) != 0 {
		return nil, NotRegularFileError(filePath)
	}

	data, err := ioutil.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	ros := New(true, filePath)
	err = yaml.Unmarshal(data, ros)
	if err != nil {
		return nil, err
	}

	ire, err := ros.Cfg.Ign.Compile()
	if nil != err {
		return nil, err
	}
	ros.Cfg.ire = *ire

	// initialize absentee list
	for mem := range ros.Mem {
		inc := true
		// if files previously added to roster are now on the ignore list, skip
		// adding them to the absentee list
		for _, ire := range ros.Cfg.ire {
			if ire.MatchString(mem) {
				inc = false
				break
			}
		}
		if inc {
			ros.abs[mem] = true
		}
	}

	return ros, nil
}

// Write formats and writes the receiver Roster ros's configuration and member
// data to disk. Returns an error if formatting or writing fails.
func (ros *Roster) Write() error {
	data, err := yaml.Marshal(ros)
	if nil != err {
		return err
	}
	return ioutil.WriteFile(ros.path, data, Permissions)
}

// Status checks if the given file path exists in the index and returns its
// corresponding Status struct and true. If the file path does not exist, it
// returns the unique NoStatus struct and false.
func (ros *Roster) Status(filePath string) (Status, bool) {
	ros.memlk.Lock()
	defer ros.memlk.Unlock()
	if stat, ok := ros.Mem[filePath]; ok {
		return stat, true
	} else {
		return NoStatus(), false
	}
}

// Keep returns whether or not a file with the given path should be considered
// candidate for indexing. Directories, files matching an ignore pattern, and
// the roster index file itself all return false.
func (ros *Roster) Keep(filePath string, info os.FileInfo) bool {
	if uint32(info.Mode()&os.ModeType) != 0 {
		return false
	}
	if filepath.Base(filePath) == filepath.Base(ros.path) {
		return false
	}
	for _, ire := range ros.Cfg.ire {
		if ire.MatchString(filePath) {
			return false
		}
	}
	return true
}

// Changed determines if the given file path and os.FileInfo already exists in
// the roster index, computes the Status struct for the given file, and returns
// whether it is a new file, whether the Status info has changed, and what the
// new Status is, along with any error encountered.
func (ros *Roster) Changed(root string, relPath string, info os.FileInfo) (
	new bool, changed bool, stat Status, err error,
) {
	prev, ok := ros.Status(relPath)
	stat, err = MakeStatus(root, relPath, info)
	if ok && prev.Valid() {
		return false, !prev.Equals(stat, ros.Cfg.Ver), stat, err
	} else {
		return true, false, stat, err
	}
}

// Update replaces the Status struct associated with a given file path in the
// roster index if valid.
func (ros *Roster) Update(filePath string, stat Status) error {
	if !stat.Valid() {
		return errors.New("invalid member status")
	}

	ros.memlk.Lock()
	ros.Mem[filePath] = stat
	ros.memlk.Unlock()

	ros.abslk.Lock()
	if _, ok := ros.abs[filePath]; ok {
		delete(ros.abs, filePath)
	}
	ros.abslk.Unlock()

	return nil
}

// Expel removes the given file path from the receiver Roster ros.
func (ros *Roster) Expel(filePath string) {
	ros.memlk.Lock()
	defer ros.memlk.Unlock()
	if _, ok := ros.Mem[filePath]; ok {
		delete(ros.Mem, filePath)
	}
}

// Absentees returns a list of files that remain in the receiver Roster ros's
// list of missing files.
func (ros *Roster) Absentees() []string {
	abs := make([]string, len(ros.abs))
	i := 0
	for s := range ros.abs {
		abs[i] = s
		i++
	}
	return abs
}
