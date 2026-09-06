package buildinfo

import "fmt"

var (
	Version = "N/A"
	Date    = "N/A"
	Commit  = "N/A"
)

func Print() {
	fmt.Printf("Build version: %s\n", Version)
	fmt.Printf("Build date: %s\n", Date)
	fmt.Printf("Build commit: %s\n", Commit)
}
