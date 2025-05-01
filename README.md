# AIP Profile Helper

This utility helps set up various CLI tools with access to all your cloud accounts. For AWS, it will create AWS CLI (profiles)[https://docs.aws.amazon.com/cli/v1/userguide/cli-configure-files.html#cli-configure-files-format] for each account and role you have access to. It also creates Steampipe connectors for each profile, and an aggregate connector for each unique role.

## Usage

```
Usage:
  aiphelper [OPTIONS] <aws | azure>

Application Options:
  -V, --version       aiphelper Version
  -d, --debug         Enable debug logging
      --kion-url=     Kion URL to use for profile generation (env: KION_URL)
      --kion-apikey=  Kion API token for authentication (env: KION_APIKEY)

Help Options:
  -h, --help  Show this help message

Available commands:
  aws    Initialize AWS
  azure  Initialize Azure

[aws command options]
      Account Source Options (exactly one required):
          --from-sso     Use AWS Identity Center to get account list
          --from-kion    Use Kion API to get account list

          --sso-start-url=  AWS SSO Start URL (default: https://aggie-innovation-platform.awsapps.com/start)
          --sso-region=     AWS SSO Region (default: us-east-2)
          --sso-role-name=  SSO Role To Assume (must be the same across all accounts) (default: AdministratorAccess)
          --regions=        Comma-separated list of regions to tell Steampipe to connect to (default: uses same search order as aws cli)
          --accounts=       Comma-separated list of accounts to tell Steampipe to connect to (default: all accounts assigned to you through SSO)
          --output-format=  Output format for AWS CLI (default: json)
          --default-region= Default region for AWS CLI operations (default: us-east-1)

[azure command options]
          --tenant-id=       Azure Tenant ID (default: 68f381e3-46da-47b9-ba57-6f322b8f0da1)
      -g, --enum-mgmt-group  Enumerate Azure Management Group descendants for a list of Subscriptions
          --root-group=      management group IDs to begin search for subscriptions (default: tamu)
          --auth-method=     Authentication method to use. Options: [environment, cli, managed-identity, device-code, default] (default: default)
```

Example usage:

```
# Default regions
aiphelper aws

# Specific regions
aiphelper aws --regions us-east-1,us-east-2

# AWS CLI use
aws ec2 describe-instances --profile div_dept_my_account_001_readonlyaccess --filters "Name=tag:Environment,Values=test"

# Steampipe use
steampipe query 'select * from aws_div_dept_my_account_001_readonlyaccess.ec2_instance where tags["Environment"] = "test"'

# Steampipe with aggregate connector
steampipe query 'select * from aws_all_readonlyaccess.ec2_instance where tags["Environment"] = "test"'

```

## AWS

`aiphelper` will create an aws profile for each account and role you have access to based on the account's display name and the role name. The profile name will be in the format `<account_name>_<role_name>` where both account and role names are normalized to be lowercase with underscores, and truncated to 63 characters.

For example, if you have access to the account `Div Dept My Account 002` with two roles, `AdministratorAccess` and `ReadOnlyAccess`, two profiles will be created: `div_dept_my_account_002_administratoraccess` and `div_dept_my_account_002_readonlyaccess`.

The profiles will be created in the AWS CLI config file, which is typically located at `~/.aws/config`. If you have custom profiles, they will be preserved. `aiphelper` will only manage the file contents between its file markers, `### AIPHELPER_MARKER_[START|END] ###`


### Kion Integration

`aiphelper` can be used to generate AWS profiles from Kion that use the `kion-cli` to generate short-term credentials transparently. This is useful for directly using the AWS CLI with Kion-managed accounts, as well as with tools that use the AWS CLI config file, such as Steampipe. Kion is the default account source, but you can explicitly specify it with the `--from-kion` option.

To use this feature, you must have the `kion-cli` tool installed and configured. See the [Kion CLI documentation](https://github.com/kionsoftware/kion-cli) for more information.

The Kion URL and API key can be set using the `--kion-url` and `--kion-apikey` options or the environment variables `KION_URL` and `KION_APIKEY`. `aiphelper` does not yet support sharing Kion API credentials with `kion-cli`, but this feature is planned for a future release.

### AWS Identity Center (SSO)

`aiphelper` can be used to generate AWS profiles from AWS Identity Center. AWS Identity Center is not used for AIP customer access, but is still used for some internal and staff accounts. Please use the Kion integration if you are an AIP customer.

To use Identity Center as the account source, use the `--from-sso` option. This will use the AWS CLI SSO configuration to generate profiles for each account and role you have access to.

However, unlike with the Kion integration, the SSO integration only creates supports a single role per account, specified by the `--sso-role-name` option. Two profiles will be created per account in the formats `aws_<account_name>` and `aws_<account_number>`. 

For steampipe, A single aggregate connector `aws_all` will be created with one of each AWS account, using the `aws_<accountnumber>` connectors.

If you already have an AWS CLI SSO token that matches the SSO URL and region, it will be used. Otherwise, a new device flow authentication will be started using the SSO parameters, and the token will be cached to disk for further AWS CLI operations.

## Azure

`aiphelper` requires Azure to already be authenticated and by default will use a series of locations to look for credentials: environment variables, a managed identity, or the azure CLI. To learn more, see [DefaultAzureCredential](https://pkg.go.dev/github.com/Azure/azure-sdk-for-go/sdk/azidentity#readme-defaultazurecredential).

The easiest way to get started is to use the `az login` command to authenticate the azure CLI with Azure.

If you need to specify an authentication method, such as to use CLI or ENV credentials on an Azure VM with a managed identity, use the `--auth-method` option.

## Steampipe

### AWS

`aiphelper` will create one Steampipe connector for each AWS profile for each region specified, defaulting to the AWS CLI default region search order. The connector names will be in the format `aws_<account_name>_<role_name>` where both account and role names are normalized to be lowercase with underscores, and truncated to 63 characters.

When using Kion as an account source, aggregate connectors will be created for each unique role. The connector names will be in the format `aws_all_<role_name>` where the role name is normalized to be lowercase with underscores, and truncated to 63 characters. The aggregate connectors will allow you to query multiple accounts at once, limited to the accounts that role has access to.

For example, if you have access to the accounts `Div Dept My Account 001` and `Div Dept My Account 002` with a `ReadOnlyAccess` role, an aggregate connector will be created as `aws_all_readonlyaccess` which will use both the `aws_div_dept_my_account_001_readonlyaccess` and `aws_div_dept_my_account_002_readonlyaccess` connectors.

If you are using AWS Identity Center as the account source, only one aggregate connector will be created, `aws_all`, which will use all the AWS accounts you have access to. 

The Steampipe configuration file will be created in the Steampipe config directory, which is typically located at `~/.steampipe/config/aws.spc`. If you have custom connectors or settings, they will be preserved. `aiphelper` will only manage the file contents between its file markers, `### AIPHELPER_MARKER_[START|END] ###`

### Azure

`aiphelper` will create a steampipe connector for each Azure subscription it discovers. It will also create an aggregate connector `azure_all` with every Azure subscription.

### Performance

It is highly recommended to limit the number of connectors and tables being queried to limit the number of API calls steampipe must make. This is especially important for the aggregate connectors. For these, it will be imperative to only fetch the precise columns you need from the tables. Do not fetch all columns if you want your computer to stay calm.
