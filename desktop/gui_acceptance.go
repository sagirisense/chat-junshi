//go:build guiacceptance

package main

import (
	"os"
	"path/filepath"
)

// GUI acceptance builds cannot open production or source checkout user data.
func init() {
	executable,err:=os.Executable()
	if err!=nil { panic(err) }
	if err=os.Setenv("JUNSHI_DATA_ROOT",filepath.Join(filepath.Dir(executable),"gui-acceptance-data")); err!=nil { panic(err) }
}
