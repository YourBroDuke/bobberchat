# Terraform Bootstrap Module

This module creates the Azure Storage Account and Resource Group that will store Terraform state files for all subsequent BobberChat infrastructure deployments (staging and production environments).

## Overview

This bootstrap module is a **one-time manual Terraform run** that must be executed BEFORE deploying any other Terraform modules or environments. It provisions:

- **Azure Resource Group**: Dedicated resource group for Terraform state backend (`rg-bobberchat-tfstate`)
- **Azure Storage Account**: Geo-redundant storage account with strict security settings
- **Blob Container**: Private blob container for state files with versioning enabled
- **Versioning**: Enabled on the storage account to protect against accidental state corruption

## Prerequisites

- Azure CLI installed and authenticated
- Terraform v1.0 or later
- Appropriate Azure subscription permissions to create resource groups and storage accounts

## Quick Start

### 1. Initialize Terraform (without backend, for initial setup)

```bash
cd deploy/terraform/bootstrap/
terraform init -backend=false
```

### 2. Validate the configuration

```bash
terraform validate
```

### 3. Review the resources to be created

```bash
terraform plan
```

### 4. Create the backend resources

```bash
terraform apply
```

Terraform will output the storage account name and container name. Record these for use in subsequent modules.

## Outputs

After successful application, this module exports:

| Output | Description |
| --- | --- |
| `resource_group_name` | Name of the resource group for state backend |
| `storage_account_name` | Name of the storage account for state files |
| `container_name` | Name of the blob container for state files |
| `storage_account_id` | Azure resource ID of the storage account |

Example usage in dependent modules:

```bash
terraform init \
  -backend-config="resource_group_name=rg-bobberchat-tfstate" \
  -backend-config="storage_account_name=$(terraform output -raw storage_account_name)" \
  -backend-config="container_name=$(terraform output -raw container_name)" \
  -backend-config="key=staging.tfstate"
```

## Variables

This module exposes the following variables with sensible defaults:

| Variable | Default | Description |
| --- | --- | --- |
| `resource_group_name` | `rg-bobberchat-tfstate` | Name of the Azure resource group |
| `location` | `East US` | Azure region for resources |
| `storage_account_name` | `stbobberchatstate` | Name of the storage account (must be globally unique) |
| `container_name` | `tfstate` | Name of the blob container |

To override defaults, create a `terraform.tfvars` file:

```hcl
resource_group_name = "my-custom-rg"
storage_account_name = "mycustomstateaccount"
location             = "US East 2"
```

## Security Configuration

This module enforces the following security controls on the storage account:

- **HTTPS only**: All traffic is encrypted in transit
- **Shared access keys disabled**: Access controlled via Azure AD and managed identities
- **Public network access disabled**: Storage account is not publicly accessible
- **TLS 1.2 minimum**: Enforces modern TLS versions
- **Geo-redundant storage (GRS)**: Automatic failover to secondary region
- **Versioning enabled**: All state file versions are retained for audit trails

## Important Notes

⚠️ **Do NOT delete this backend** once created. Destroying these resources will make Terraform unable to access state files for existing environments.

⚠️ **This is a one-time setup**: Run this module once, then use its outputs in subsequent Terraform modules for staging and production environments.

⚠️ **Storage account names are globally unique**: If the default name `stbobberchatstate` is already taken, override the `storage_account_name` variable with a unique name.

## Troubleshooting

### Error: "InvalidResourceName"

The storage account name contains invalid characters or is already taken. Edit `terraform.tfvars` and set a unique `storage_account_name` value (lowercase alphanumerics only, 3-24 characters).

### Error: "ResourceGroupAlreadyExists"

Another resource group with the same name exists. Either:
1. Change the `resource_group_name` variable, or
2. Use `terraform import` to adopt the existing resource group into this state

### Successfully Applied but Can't Access Storage

Verify your Azure CLI authentication and permissions:

```bash
az account show
az role assignment list --assignee $(az account show --query user.name -o tsv)
```

## Next Steps

Once this module is successfully applied:

1. Record the outputs from `terraform output`
2. Proceed to deploying the network module (`deploy/terraform/network/`)
3. Use the storage account name and container name in the `-backend-config` flags for subsequent module deployments

## Related Documentation

- Terraform AzureRM Provider: https://registry.terraform.io/providers/hashicorp/azurerm/latest
- Azure Storage Account: https://docs.microsoft.com/en-us/azure/storage/common/storage-account-overview
- Terraform Backend Configuration: https://www.terraform.io/language/settings/backends/configuration
