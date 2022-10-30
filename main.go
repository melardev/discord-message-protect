package main

import (
	"flag"
)

func main() {
	args := Args{}
	flag.StringVar(&args.ConfigPath, "config", "", "configuration file path")
	flag.Parse()

	app = GetApplication(&args)

	app.Run()
}
