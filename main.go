package main

import "github.com/marwan/aoa/cmd"

func main() {
	cmd.SetBuildAssets(BuildAssets)
	cmd.Execute()
}
