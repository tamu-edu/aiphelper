package aws

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	ssotypes "github.com/aws/aws-sdk-go-v2/service/sso/types"
	"github.com/tamu-edu/aiphelper/utils"
)

// KionTimeField represents a time with validity flag in Kion API responses
type KionTimeField struct {
	Time  time.Time `json:"Time"`
	Valid bool      `json:"Valid"`
}

// KionCloudAccessRole represents a cloud access role from Kion API
type KionCloudAccessRole struct {
	AccountAlias        string        `json:"account_alias"`
	AccountID           int           `json:"account_id"`
	AccountName         string        `json:"account_name"`
	AccountNumber       string        `json:"account_number"`
	AccountType         string        `json:"account_type"`
	AccountTypeID       int           `json:"account_type_id"`
	ApplyToAllAccounts  bool          `json:"apply_to_all_accounts"`
	AwsIamPath          string        `json:"aws_iam_path"`
	AwsIamRoleName      string        `json:"aws_iam_role_name"`
	CloudAccessRoleType string        `json:"cloud_access_role_type"`
	CreatedAt           KionTimeField `json:"created_at"`
	DeletedAt           KionTimeField `json:"deleted_at"`
	FutureAccounts      bool          `json:"future_accounts"`
	ID                  int           `json:"id"`
	LongTermAccessKeys  bool          `json:"long_term_access_keys"`
	Name                string        `json:"name"`
	ProjectID           int           `json:"project_id"`
	ShortTermAccessKeys bool          `json:"short_term_access_keys"`
	UpdatedAt           KionTimeField `json:"updated_at"`
	WebAccess           bool          `json:"web_access"`
}

// KionCloudAccessRolesResponse represents the response from the Kion API
type KionCloudAccessRolesResponse struct {
	Data   []KionCloudAccessRole `json:"data"`
	Status int                   `json:"status"`
}

// GetAccountsFromKion retrieves AWS account information from Kion API
func GetAccountsFromKion(kionURL, kionApikey string) ([]AWSAccountInfo, error) {
	if kionURL == "" {
		return nil, errors.New("kion URL is required when using --from-kion")
	}

	if kionApikey == "" {
		return nil, errors.New("kion token is required when using --from-kion")
	}

	endpoint := fmt.Sprintf("%s/api/v3/me/cloud-access-role", kionURL)

	req, err := http.NewRequest("GET", endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}

	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", kionApikey))

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to call Kion API: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("kion API returned non-200 status code: %d", resp.StatusCode)
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %v", err)
	}

	var kionResp KionCloudAccessRolesResponse
	if err := json.Unmarshal(body, &kionResp); err != nil {
		utils.Debug("Kion API response: %s\n", string(body))
		return nil, fmt.Errorf("failed to unmarshal response: %v", err)
	}

	var accounts []AWSAccountInfo
	// uniqueAccounts := make(map[string]bool)

	for _, role := range kionResp.Data {
		// Skip deleted accounts or non-AWS accounts
		if role.DeletedAt.Valid || role.AccountTypeID != 1 {
			continue
		}

		// Only add each account once (we might see multiple roles per account)
		// if _, exists := uniqueAccounts[role.AccountNumber]; exists {
		// 	continue
		// }
		// uniqueAccounts[role.AccountNumber] = true

		normalizedAccountName := utils.SnakeCase(fmt.Sprintf("%s_%s", role.AccountName, role.Name))

		if len(normalizedAccountName) > 63 {
			chars_to_cut := len(normalizedAccountName) - 63

			// Get individual snake_case parts
			accountNameSnake := utils.SnakeCase(role.AccountName)
			roleNameSnake := utils.SnakeCase(role.Name)

			// Calculate how much to trim from each part (evenly distribute the cut)
			accountCut := chars_to_cut / 2
			roleCut := chars_to_cut - accountCut

			// Make sure we don't trim more characters than available
			if accountCut > len(accountNameSnake)-3 {
				accountCut = len(accountNameSnake) - 3
				roleCut = chars_to_cut - accountCut
			}
			if roleCut > len(roleNameSnake)-3 {
				roleCut = len(roleNameSnake) - 3
				accountCut = chars_to_cut - roleCut
			}

			// Apply the trimming
			trimmedAccountName := accountNameSnake
			trimmedRoleName := roleNameSnake
			if accountCut > 0 {
				trimmedAccountName = accountNameSnake[:len(accountNameSnake)-accountCut]
			}
			if roleCut > 0 {
				trimmedRoleName = roleNameSnake[:len(roleNameSnake)-roleCut]
			}

			// Create the new normalized name with trimmed parts
			normalizedAccountName = fmt.Sprintf("%s_%s", trimmedAccountName, trimmedRoleName)

		}

		accountId := role.AccountNumber
		accountName := role.AccountName

		accountInfo := AWSAccountInfo{
			NormalizedAccountName:   normalizedAccountName,
			KionCloudAccessRoleName: role.Name,
			AccountInfo: ssotypes.AccountInfo{
				AccountId:    &accountId,
				AccountName:  &accountName,
				EmailAddress: nil,
			},
		}

		accounts = append(accounts, accountInfo)
	}

	return accounts, nil
}
