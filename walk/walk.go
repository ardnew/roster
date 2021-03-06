// Package walk implements the multi-threaded file traversal logic that will
// coordinate identifying new and changed files recursively throughout a
// directory tree.
package walk

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"github.com/ardnew/roster/file"
)

// Info stores a unique description of a complete file path (relative) along
// with its os.FileInfo obtained from filepath.Walk.
type Info struct {
	path string
	info os.FileInfo
}

// Walk traverses a directory tree recursively, constructing a roster index file
// along the way, and returns a list of all new files discovered and a list of
// all existing files that have changed since they were last recorded.
func Walk(filePath string, roster *file.Roster) (new []string, mod []string, del []string) {

	new = []string{}
	mod = []string{}
	del = []string{}

	// funnel the worker goroutines' output into shared slices of strings
	funnelNew := make(chan string)
	funnelMod := make(chan string)

	funnel := func(ret *[]string, grp *sync.WaitGroup, fun chan string) {
		for s := range fun {
			*ret = append(*ret, s)
		}
		grp.Done()
	}

	waitNew := sync.WaitGroup{}
	waitNew.Add(1)
	go funnel(&new, &waitNew, funnelNew)

	waitMod := sync.WaitGroup{}
	waitMod.Add(1)
	go funnel(&mod, &waitMod, funnelMod)

	// use the number of threads specified in roster file's configuration
	threads := roster.Cfg.Rt.Thr
	if file.RuntimeThreadsNoLimit == threads {
		// if 0 threads (default), use number of CPU cores
		threads = runtime.NumCPU()
	}

	// unbuffered channel, so we have to ensure all receivers are ready before
	// filepath.Walk begins sending files to the channel.
	var work sync.WaitGroup
	queue := make(chan Info)

	// spawn worker goroutines to process multiple files simultaneously
	for i := 0; i < threads; i++ {
		go func(w *sync.WaitGroup, d string, q chan Info, r *file.Roster, n, m chan string) {
			for in := range q {
				// determine if the file is new or changed
				if new, mod, stat, err := r.Changed(d, in.path, in.info); nil != err {
					fmt.Printf("error: Changed(): %s: %s\n", err.Error(), in.path)
				} else {
					// update the roster index (in-memory) with current file attributes
					if err := r.Update(in.path, stat); nil != err {
						fmt.Printf("error: Update(): %s: %s\n", err.Error(), in.path)
					} else {
						if new {
							n <- in.path
						} else if mod {
							m <- in.path
						}
					}
				}
				w.Done()
			}
		}(&work, filePath, queue, roster, funnelNew, funnelMod)
	}

	filepath.Walk(filePath,
		func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			relPath := strings.TrimPrefix(path, filepath.Clean(filePath)+string(os.PathSeparator))
			// check if this file is ignored
			if roster.Keep(relPath, info) {
				work.Add(1)
				queue <- Info{relPath, info}
			}
			return nil
		})

	// notify the worker goroutines to clean up, no more files are coming
	close(queue)
	// ensure all of the worker goroutines have finished
	work.Wait()

	// notify the funnel workers to terminate
	close(funnelNew)
	close(funnelMod)

	// ensure all output strings have been appended
	waitNew.Wait()
	waitMod.Wait()

	// finally, remove all missing files from the roster
	del = roster.Absentees()
	for _, s := range del {
		roster.Expel(s)
	}

	return new, mod, del
}
