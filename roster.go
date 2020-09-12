package main

import (
	"flag"
	"fmt"
	"path/filepath"
	"sort"

	"github.com/ardnew/roster/file"
	"github.com/ardnew/roster/walk"
	"github.com/ardnew/version"
)

func init() {
	version.ChangeLog = []version.Change{
		{
			Package: "roster",
			Version: "0.1.0",
			Date:    "Sept 10, 2020",
			Description: []string{
				`initial commit`,
			},
		},
	}
}

const (
	rosterFileNameDefault = ".roster.yml"
	updateRosterDefault   = false
)

func main() {

	var (
		rosterFileName string
		updateRoster   bool
	)

	flag.StringVar(&rosterFileName, "f", rosterFileNameDefault, "roster file name")
	flag.BoolVar(&updateRoster, "u", updateRosterDefault, "update roster with scan results")
	flag.Parse()

	if len(flag.Args()) == 0 {
		fmt.Printf("error: no directory path(s) provided\n")
	}

	for _, dir := range flag.Args() {
		path := filepath.Join(dir, rosterFileName)
		ros, err := file.Parse(path)
		if nil != err {
			fmt.Printf("error: file.Parse(): %s (skipping)\n", err.Error())
			continue
		}

		new, mod := walk.Walk(dir, ros)

		sort.Strings(new)
		for _, s := range new {
			fmt.Printf("+ %s\n", s)
		}

		sort.Strings(mod)
		for _, s := range mod {
			fmt.Printf("%s\n", s)
		}

		if updateRoster {
			if err := ros.Write(); nil != err {
				fmt.Printf("marshal error: %s\n", err)
			}
		}
	}

}
