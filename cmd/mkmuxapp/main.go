// Command mkmuxapp zips a stage directory into a muOS .muxapp archive.
//
//	mkmuxapp <stage-dir> <out.muxapp>
package main

import (
	"fmt"
	"os"

	"github.com/amtypaldos/birdroms-handheld/internal/muxapp"
)

func main() {
	if len(os.Args) != 3 {
		fmt.Fprintln(os.Stderr, "usage: mkmuxapp <stage-dir> <out.muxapp>")
		os.Exit(2)
	}
	if err := muxapp.Build(os.Args[1], os.Args[2]); err != nil {
		fmt.Fprintln(os.Stderr, "mkmuxapp:", err)
		os.Exit(1)
	}
}
