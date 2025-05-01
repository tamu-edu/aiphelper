package main

import (
	"fmt"
	"os"

	"github.com/jessevdk/go-flags"
	"github.com/tamu-edu/aiphelper/aws"
	"github.com/tamu-edu/aiphelper/azure"
	"github.com/tamu-edu/aiphelper/utils"
)

// https://lightstep.com/blog/getting-real-with-command-line-arguments-and-goflags/
// var (
// 	arguments = new(config.Parameters)
// )

var Version = "development"

var opts struct {
	Version    bool   `long:"version" short:"V" description:"aiphelper Version"`
	Debug      bool   `long:"debug" short:"d" description:"Enable debug logging"`
	KionUrl    string `long:"kion-url" description:"Kion URL to use for profile generation" env:"KION_URL"`
	KionApikey string `long:"kion-apikey" description:"Kion API token for authentication" env:"KION_APIKEY"`
}

func main() {
	p := flags.NewParser(&opts, flags.HelpFlag|flags.PassDoubleDash)

	aws.AddCommand(p)
	azure.AddCommand(p)

	_, err := p.Parse()

	// Set the debug flag in the utils package
	utils.DebugEnabled = opts.Debug

	// Store global options
	utils.SetGlobalOption("kion-url", opts.KionUrl)
	utils.SetGlobalOption("kion-apikey", opts.KionApikey)

	if opts.Version {
		fmt.Printf("Version: %s\n", Version)
		os.Exit(0)
	}

	if err != nil {
		fmt.Println(err)
		os.Exit(-1)
	}

	switch p.Active.Name {
	case "aws":
		aws.Init()
	case "azure":
		azure.Init()
	}
}
