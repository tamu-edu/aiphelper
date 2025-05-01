package aws

import (
	"errors"
	"log"
	"strings"

	"github.com/jessevdk/go-flags"
	"github.com/tamu-edu/aiphelper/utils"
)

type Regions struct {
	All []string
}

type Accounts struct {
	All []string
}

var (
	options *Options
)

type Options struct {
	SSOStartURL   string    `long:"sso-start-url" default:"https://aggie-innovation-platform.awsapps.com/start" description:"AWS SSO Start URL"`
	SSORegion     string    `long:"sso-region" default:"us-east-2" description:"AWS SSO Region"`
	SSORoleName   string    `long:"sso-role-name" default:"AdministratorAccess" description:"SSO Role To Assume (must be the same across all accounts)"`
	Regions       Regions   `long:"regions" default:"" description:"Comma-separated list of regions to tell Steampipe to connect to (default: uses same search order as aws cli)"`
	Accounts      *Accounts `long:"accounts" default:"" description:"Comma-separated list of accounts to tell Steampipe to connect to (default: all accounts assigned to you through SSO)"`
	DefaultFormat string    `long:"output-format" default:"json" description:"Output format for AWS CLI"`
	DefaultRegion string    `long:"default-region" default:"us-east-1" description:"Default region for AWS CLI operations"`
	FromKion      bool      `long:"from-kion" description:"Use Kion API to get AWS account list (default)" group:"account-source"`
	FromSSO       bool      `long:"from-sso" description:"Use AWS Identity Center to get account list" group:"account-source"`
}

func AddCommand(p *flags.Parser) {
	options = &Options{}
	options.FromKion = true // Default to Kion

	_, err := p.AddCommand("aws", "Initialize AWS", "Initialize AWS config and Steampipe connections", options)
	if err != nil {
		log.Fatalf("Failed to add AWS command: %v", err)
	}
}

// Validate checks that the options are valid
func (o *Options) Execute(args []string) error {
	// Ensure exactly one account source is selected
	if o.FromKion == o.FromSSO {
		return errors.New("you must specify exactly one account source: either --from-kion or --from-sso")
	}

	// If using Kion, ensure URL and API key are provided

	if o.FromKion {
		// Get Kion URL and API key from global options
		kionURL := utils.GetGlobalOption("kion-url")
		kionApikey := utils.GetGlobalOption("kion-apikey")

		if kionURL == "" {
			return errors.New("--kion-url is required when using --from-kion")
		}
		if kionApikey == "" {
			return errors.New("--kion-apikey is required when using --from-kion")
		}
	}

	return nil
}

func (r *Regions) UnmarshalFlag(arg string) error {
	// if len(arg) == 0 {
	// 	r.All = []string{}
	// 	return
	// }
	log.Println("arg: ", arg)
	regions := strings.Split(arg, ",")

	r.All = regions

	return nil
}

func (a *Accounts) UnmarshalFlag(arg string) error {
	if arg == "" {
		a.All = []string{}
		return nil
	}
	var tempValue = utils.SplitArgumentParser(arg)
	if len(tempValue) == 0 {
		return errors.New("invalid account list")
	}
	a.All = tempValue
	return nil
}
