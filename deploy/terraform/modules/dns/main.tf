# Azure DNS Zone Module
# Manages DNS zone for custom domain (optional, for future use)

# Create the DNS zone for the custom domain
resource "azurerm_dns_zone" "main" {
  count               = var.create_dns_zone ? 1 : 0
  name                = var.domain_name
  resource_group_name = var.resource_group_name

  tags = var.tags
}
