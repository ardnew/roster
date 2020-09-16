package roster

import (
	"errors"
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

type Handler func(string)

type Taker struct {
	NewFile Handler
	ModFile Handler
	DelFile Handler
}

var (
	DefaultTaker = Taker{
		NewFile: func(filePath string) { fmt.Println("+ " + filePath) },
		ModFile: func(filePath string) { fmt.Println(filePath) },
		DelFile: func(filePath string) { fmt.Println("- " + filePath) },
	}
	SkipTaker = Taker{
		NewFile: Skip,
		ModFile: Skip,
		DelFile: Skip,
	}
	Skip = Handler(nil)
)

func Take(take Taker, filename string, update bool, path ...string) error {

	if len(path) == 0 {
		return errors.New("no directory path(s) provided")
	}

	for _, dir := range path {
		path := filepath.Join(dir, filename)
		ros, err := file.Parse(path)
		if nil != err {
			return fmt.Errorf("file.Parse(): %s\n", err.Error())
		}

		new, mod, del := walk.Walk(dir, ros)

		sort.Strings(new)
		if take.NewFile != nil {
			for _, s := range new {
				take.NewFile(s)
			}
		}

		sort.Strings(mod)
		if take.ModFile != nil {
			for _, s := range mod {
				take.ModFile(s)
			}
		}

		sort.Strings(del)
		if take.DelFile != nil {
			for _, s := range del {
				take.DelFile(s)
			}
		}

		if update {
			if err := ros.Write(); nil != err {
				return fmt.Errorf("ros.Write(): %s\n", err)
			}
		}
	}
	return nil
}
