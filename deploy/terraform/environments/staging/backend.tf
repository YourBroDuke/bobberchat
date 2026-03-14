terraform {
  backend "azurerm" {}
}

# Initialize with:
# terraform init \
#   -backend-config="resource_group_name=rg-bobberchat-tfstate" \
#   -backend-config="storage_account_name=<FROM_BOOTSTRAP_OUTPUT>" \
#   -backend-config="container_name=tfstate" \
#   -backend-config="key=staging.tfstate"
