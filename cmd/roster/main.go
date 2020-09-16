package main

import (
	"flag"
	"fmt"

	"github.com/ardnew/roster"
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

	if err := roster.Take(roster.DefaultTaker, rosterFileName, updateRoster, flag.Args()...); nil != err {
		fmt.Printf("error: %s\n", err)
	}
}
