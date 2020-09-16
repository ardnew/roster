package main

import (
	"flag"
	"fmt"
	"os"

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

const (
	exitCodeErr = 125
	exitCodeNew = 1 << 0
	exitCodeMod = 1 << 1
	exitCodeDel = 1 << 2
)

func main() {

	var (
		rosterFileName string
		updateRoster   bool
	)

	flag.StringVar(&rosterFileName, "f", rosterFileNameDefault, "roster file name")
	flag.BoolVar(&updateRoster, "u", updateRosterDefault, "update roster with scan results")
	flag.Parse()

	var new, mod, del uint
	if err := roster.Take(roster.Taker{
		NewFile: func(filePath string) { new++; roster.DefaultNewHandler(filePath) },
		ModFile: func(filePath string) { mod++; roster.DefaultModHandler(filePath) },
		DelFile: func(filePath string) { del++; roster.DefaultDelHandler(filePath) },
	}, rosterFileName, updateRoster, flag.Args()...); nil != err {
		fmt.Printf("error: %s\n", err)
		os.Exit(exitCodeErr)
	}

	exitCode := 0
	if new > 0 {
		exitCode |= exitCodeNew
	}
	if mod > 0 {
		exitCode |= exitCodeMod
	}
	if del > 0 {
		exitCode |= exitCodeDel
	}
	os.Exit(exitCode)
}
