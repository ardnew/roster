[docimg]:https://godoc.org/github.com/ardnew/roster?status.svg
[docurl]:https://godoc.org/github.com/ardnew/roster
[repimg]:https://goreportcard.com/badge/github.com/ardnew/roster
[repurl]:https://goreportcard.com/report/github.com/ardnew/roster

# roster
#### Check which files have changed using configurable directory index file

[![GoDoc][docimg]][docurl] [![Go Report Card][repimg]][repurl]

## Synopsis

`roster` uses a combined index and configuration file in YAML format (the "roster index") to record the file size, last modification time, permissions, and checksum (using the very-fast xxHash algorithm) of files in a given directory tree. 

The roster index contains configuration parameters that control which, if any, of the attributes mentioned above are used when determining if files have changed since they were last recorded. It can also define ignore patterns to exclude files and directories from the index, as well as number of threads (goroutines) to spawn concurrently for analyzing file attributes (by default, it uses the number of CPU cores available).

The program will first output the list of newly discovered files that do not exist in the index, one per line, with each line prefixed by the string `+ `.

Following the list of new files, the list of all files that have changed since they were last recorded is then printed, also one per line, without any string prefix.

The following command-line flags are recognized:

```
$ roster -h
Usage of roster:
  -f string
    	roster file name (default ".roster.yml")
  -u	update roster with scan results
```

## Format

The following is an example of the default roster index file on this project directory, configured to ignore `git` metadata, inspect all attributes when comparing files, and to use all CPU cores when analyzing files:

```yaml
config:
    runtime:
        threads: 0
        maxdepth: 0
    verify:
        filesize: true
        permissions: true
        lastmodtime: true
        checksum: true
    ignore:
        - '*.git/*'
        - '*.git/*/*'
        - '*.git/*/*/*'
        - '*.git/*/*/*/*'
        - '*.git/*/*/*/*/*'
members:
    LICENSE:
        size: 1063
        perm: 420
        last: 1599752882
        hash: fd3c429a8324bac
    README.md:
        size: 1487
        perm: 420
        last: 1599867356
        hash: f944851ccad9bd13
    file/file.go:
        size: 8400
        perm: 420
        last: 1599869506
        hash: d1419cb484331dc6
    roster.go:
        size: 1301
        perm: 420
        last: 1599867737
        hash: 1156ef926fc75b83
    walk/walk.go:
        size: 3041
        perm: 420
        last: 1599868623
        hash: 817ca7048fd84371
```

Note the awkward ignore patterns for git are because of the Go filepath globbing requirements, as the pattern has to match the entire path, not just a substring. This could obviously be improved.

