package aws

import (
	"bytes"
	"context"
	"crypto/sha1"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"text/template"
	"time"

	"golang.org/x/exp/slices"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sso"
	ssotypes "github.com/aws/aws-sdk-go-v2/service/sso/types"
	ssooidc "github.com/aws/aws-sdk-go-v2/service/ssooidc"
	ssooidctypes "github.com/aws/aws-sdk-go-v2/service/ssooidc/types"
	"github.com/pkg/browser"

	"github.com/tamu-edu/aiphelper/utils"
)

//go:embed aws_config.tmpl
var awsTemplateString string

//go:embed aws_kion_config.tmpl
var awsKionTemplateString string

//go:embed steampipe.gospc
var steampipeTemplateString string

type SSOCachedCredential struct {
	StartUrl    string    `json:"startUrl"`
	Region      string    `json:"region"`
	AccessToken string    `json:"accessToken"`
	ExpiresAt   time.Time `json:"expiresAt"`
}

type AWSAccountInfo struct {
	NormalizedAccountName   string
	KionCloudAccessRoleName string
	ssotypes.AccountInfo
}

var (
	awsTemplate       *template.Template
	steampipeTemplate *template.Template

	accounts              []AWSAccountInfo
	steampipeTemplateData = SteampipeTemplateData{Marker: utils.Marker}
	awsTemplateData       = AWSTemplateData{Marker: utils.Marker}
)

type AWSTemplateData struct {
	Params      *Options
	AccountList []AWSAccountInfo
	Marker      string
}

type SteampipeRole struct {
	RoleName              string
	RoleProfileListString string
}

type SteampipeTemplateData struct {
	Regions           []string
	AccountList       []AWSAccountInfo
	AllAccountsString string
	RegionsString     string
	RoleList          []SteampipeRole
	Marker            string
}

func Init() {
	awsTemplate = template.Must(template.New("awsTemplate").Parse(awsTemplateString))
	awsKionTemplate := template.Must(template.New("awsTemplate").Parse(awsKionTemplateString))
	steampipeTemplate = template.Must(template.New("steampipeTemplate").Parse(steampipeTemplateString))

	awsTemplateData.Params = options

	var err error

	if options.FromKion {
		// Switch to the Kion template when --from-kion is used
		awsTemplate = awsKionTemplate

		// Get Kion URL and API key from global options
		kionURL := utils.GetGlobalOption("kion-url")
		kionApikey := utils.GetGlobalOption("kion-apikey")

		// Get accounts from Kion
		fmt.Println("Using Kion as the account source...")
		accounts, err = GetAccountsFromKion(kionURL, kionApikey)
		if err != nil {
			log.Fatalf("Failed to get accounts from Kion: %v", err)
		}

	} else {
		// Use existing SSO logic
		accessToken, cfg, err := authenticate()
		if err != nil {
			log.Fatalln(err)
		}

		// create sso client
		ssoClient := sso.NewFromConfig(cfg)
		// list accounts
		fmt.Print("Fetching list of all accounts from AWS Identity Center... ")

		accountPaginator := sso.NewListAccountsPaginator(ssoClient, &sso.ListAccountsInput{
			AccessToken: &accessToken,
		})

		for accountPaginator.HasMorePages() {
			x, err := accountPaginator.NextPage(context.TODO())
			if err != nil {
				fmt.Println(err)
			}
			for _, account := range x.AccountList {
				account := AWSAccountInfo{AccountInfo: account}
				if len(options.Accounts.All) > 0 && !slices.Contains(options.Accounts.All, *account.AccountId) {
					continue
				}
				fmt.Printf("Account: %s (%s)\n", *account.AccountName, *account.AccountId)
				account.NormalizedAccountName = utils.SnakeCase(*account.AccountName)
				accounts = append(accounts, account)
			}
		}
	}

	fmt.Printf("User has access to %d AWS accounts.\n", len(accounts))

	fmt.Println("Updating AWS config file with profiles.")
	updateAwsConfigFile()

	fmt.Println("Updating Steampipe AWS Plugin config file with connections.")
	updateSteampipeAwsConfigFile()

	fmt.Println("Done.")
}

func authenticate() (string, aws.Config, error) {
	// load default aws config
	cfg, err := awsconfig.LoadDefaultConfig(context.TODO(), awsconfig.WithRegion(options.SSORegion))
	if err != nil {
		fmt.Println(err)
	}

	accessToken, err := searchForSsoCachedCredentials(options.SSOStartURL, options.SSORegion)
	if err != nil {
		// create sso oidc client to trigger login flow
		ssooidcClient := ssooidc.NewFromConfig(cfg)
		if err != nil {
			fmt.Println(err)
		}
		// register your client which is triggering the login flow
		register, err := ssooidcClient.RegisterClient(context.TODO(), &ssooidc.RegisterClientInput{
			ClientName: aws.String("github.com/tamu-edu/aiphelper"),
			ClientType: aws.String("public"),
			Scopes:     []string{"sso-portal:*"},
		})
		if err != nil {
			fmt.Println(err)
		}
		// authorize your device using the client registration response
		deviceAuth, err := ssooidcClient.StartDeviceAuthorization(context.TODO(), &ssooidc.StartDeviceAuthorizationInput{
			ClientId:     register.ClientId,
			ClientSecret: register.ClientSecret,
			StartUrl:     aws.String(options.SSOStartURL),
		})
		if err != nil {
			fmt.Println(err)
		}

		// trigger OIDC login. open browser to login. begin polling for token. close tab once login is done.
		url := aws.ToString(deviceAuth.VerificationUriComplete)
		fmt.Printf("If browser is not opened automatically, please open link:\n%v\n", url)
		err = browser.OpenURL(url)
		if err != nil {
			fmt.Println(err)
		}

		// Wait for sso token
		var token *ssooidc.CreateTokenOutput

		var slowDownDelay = 5 * time.Second
		var retryInterval = 5 * time.Second //default value

		if i := deviceAuth.Interval; i > 0 {
			retryInterval = time.Duration(i) * time.Second // acceptable value from AWS
		}

		for {
			tokenInput := ssooidc.CreateTokenInput{
				ClientId:     register.ClientId,
				ClientSecret: register.ClientSecret,
				DeviceCode:   deviceAuth.DeviceCode,
				GrantType:    aws.String("urn:ietf:params:oauth:grant-type:device_code"),
			}
			newToken, err := ssooidcClient.CreateToken(context.TODO(), &tokenInput)

			if err != nil {
				// fmt.Printf("Got oidc error: %v\n", err)
				var sde *ssooidctypes.SlowDownException
				if errors.As(err, &sde) {
					retryInterval += slowDownDelay
				}

				var ape *ssooidctypes.AuthorizationPendingException
				if errors.As(err, &ape) {
					// fmt.Printf("Waiting %d seconds before trying again\n", retryInterval)
					time.Sleep(retryInterval)
					continue
				}
				log.Fatal(err)
			} else {
				token = newToken
				accessToken = *token.AccessToken
				break
			}
		}

		var now = time.Now()
		var exp = now.Add(time.Second * time.Duration(token.ExpiresIn))
		ssoCacheFile := SSOCachedCredential{
			AccessToken: accessToken,
			Region:      options.SSORegion,
			StartUrl:    options.SSOStartURL,
			ExpiresAt:   exp.UTC(),
		}

		if err := putSsoCachedCredentials(ssoCacheFile); err != nil {
			log.Printf("Error occurred writing the credentials to cache: %s", err)
		}
	} else {
		fmt.Println("Using existing access token in SSO cache")
	}

	return accessToken, cfg, nil
}

func updateAwsConfigFile() {
	var err error = nil
	homeDir, _ := os.UserHomeDir()
	awsConfigFilePath := filepath.Join(homeDir, ".aws/config")

	var awsTemplateBuffer bytes.Buffer

	awsTemplateData.AccountList = accounts
	utils.Debug("List of AWS accounts: %v\n", awsTemplateData.AccountList)
	err = awsTemplate.Execute(&awsTemplateBuffer, awsTemplateData)
	if err != nil {
		log.Fatalln(err)
	}

	utils.Debug("Writing AWS config file contents: %s\n", awsTemplateBuffer.String())
	err = utils.CreateOrReplaceInFile(awsConfigFilePath, awsTemplateBuffer.String())

	if err != nil {
		log.Fatalln(err)
	}
}

func updateSteampipeAwsConfigFile() {
	var spcTemplateBuffer bytes.Buffer
	var err error = nil

	steampipeTemplateData.AccountList = accounts

	steampipeTemplateData.RegionsString = strings.Join(options.Regions.All, "\", \"")

	// Create a map to group accounts by role
	roleToAccounts := make(map[string][]string)

	// Find all unique KionCloudAccessRoleName values and build lists of accounts for each role
	for _, account := range accounts {
		roleName := account.KionCloudAccessRoleName
		normalizedRoleName := strings.ToLower(roleName)
		re := regexp.MustCompile(`[^a-z0-9_]`)
		normalizedRoleName = re.ReplaceAllString(normalizedRoleName, "_")

		if roleName != "" { // Only process accounts with role names (from Kion)
			// Initialize the slice if this is the first account with this role
			if _, exists := roleToAccounts[normalizedRoleName]; !exists {
				roleToAccounts[normalizedRoleName] = []string{}
			}

			// Add this account's normalized name to the appropriate role's list
			roleToAccounts[normalizedRoleName] = append(roleToAccounts[normalizedRoleName], account.NormalizedAccountName)
		}
	}

	// Create role objects for the template
	steampipeTemplateData.RoleList = []SteampipeRole{}
	for roleName, accountNames := range roleToAccounts {
		// Create a string of profile names for this role
		profileListStr := ""
		for _, acctName := range accountNames {
			profileListStr += "\"" + acctName + "\", "
		}
		profileListStr = strings.Trim(profileListStr, ", ")

		// Add this role to the template data
		steampipeTemplateData.RoleList = append(steampipeTemplateData.RoleList, SteampipeRole{
			RoleName:              roleName,
			RoleProfileListString: profileListStr,
		})
	}

	// Debug logging
	fmt.Println("Found the following Kion roles and accounts:")
	for roleName, accounts := range roleToAccounts {
		fmt.Printf("  Role: %s\n", roleName)
		for _, acct := range accounts {
			fmt.Printf("    - %s\n", acct)
		}
	}

	for _, account := range accounts {
		steampipeTemplateData.AllAccountsString = steampipeTemplateData.AllAccountsString + "\"aws_" + account.NormalizedAccountName + "\", "
	}

	steampipeTemplateData.AllAccountsString = strings.Trim(steampipeTemplateData.AllAccountsString, ", ")

	err = steampipeTemplate.Execute(&spcTemplateBuffer, steampipeTemplateData)
	if err != nil {
		log.Fatalln(err)
	}

	homeDir, _ := os.UserHomeDir()
	spcFilePath := filepath.Join(homeDir, ".steampipe/config/aws.spc")

	err = utils.CreateOrReplaceInFile(spcFilePath, spcTemplateBuffer.String())
	if err != nil {
		log.Fatalln(err)
	}
}

func searchForSsoCachedCredentials(startUrl string, region string) (string, error) {
	homedir, _ := os.UserHomeDir()
	globPattern := filepath.Join(homedir, ".aws/sso/cache", "*.json")
	matches, err := filepath.Glob(globPattern)
	if err != nil {
		log.Fatalf("Failed to match %q: %v", globPattern, err)
	}

	for _, match := range matches {
		file, _ := ioutil.ReadFile(match)
		data := SSOCachedCredential{}
		if err = json.Unmarshal([]byte(file), &data); err != nil {
			log.Printf("Error: %v", err)
		} else {
			if data.StartUrl != startUrl {
				log.Println("Token does not match desired startUrl")
				continue
			}
			if data.Region != region {
				log.Println("Token does not match desired region")
				continue
			}
			if data.ExpiresAt.Before(time.Now()) {
				log.Println("Token has expired")
				continue
			}
			if len(data.AccessToken) == 0 {
				log.Println("Invalid access token")
				continue
			}
			return data.AccessToken, nil
		}
	}
	return "", errors.New("No access token found")
}

func putSsoCachedCredentials(creds SSOCachedCredential) error {
	s := creds.StartUrl
	h := sha1.New()
	h.Write([]byte(s))
	hash := hex.EncodeToString(h.Sum(nil))

	homedir, _ := os.UserHomeDir()
	cacheFile := filepath.Join(homedir, ".aws/sso/cache", fmt.Sprintf("%s.json", hash))

	if _, err := os.Stat(cacheFile); os.IsNotExist(err) {
		os.MkdirAll(filepath.Dir(cacheFile), 0755)
		os.Create(cacheFile)
	}

	f, err := os.OpenFile(cacheFile, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}

	cacheContents, _ := json.Marshal(creds)

	f.WriteString(string(cacheContents))

	if err := f.Close(); err != nil {
		return err
	}
	return nil
}
