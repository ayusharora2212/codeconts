package main

import "github.com/fossMeDaddy/codeconts/build-scripts/utils"

func main() {
	v := utils.GetCurrentVersion()
	utils.CreateTag(v)
}
