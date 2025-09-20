/*
DriftWatch - API Drift Detection CLI Tool

The tool helps development teams catch breaking changes, undocumented modifications,
and API evolution before they impact downstream consumers.
*/
package main

import "github.com/k0ns0l/driftwatch/cmd"

func main() {
	cmd.Execute()
}
