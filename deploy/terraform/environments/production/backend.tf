terraform {
  backend "azurerm" {
    # Backend configuration must be provided via -backend-config flags during init
    # Example: terraform init -backend-config="key=production.tfstate"
  }
}
