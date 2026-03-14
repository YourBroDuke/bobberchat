# Azure DNS Zone Module
# Manages DNS zone for custom domain and creates A records for application subdomains

# Create the DNS zone for the custom domain
resource "azurerm_dns_zone" "main" {
  count               = var.create_dns_zone ? 1 : 0
  name                = var.domain_name
  resource_group_name = var.resource_group_name

  tags = var.tags
}

# Create A record for staging subdomain when ingress external IP is known
resource "azurerm_dns_a_record" "staging" {
  count               = var.ingress_external_ip != "" ? 1 : 0
  name                = var.staging_subdomain
  zone_name           = azurerm_dns_zone.main[0].name
  resource_group_name = var.resource_group_name
  ttl                 = var.dns_record_ttl
  records             = [var.ingress_external_ip]

  tags = var.tags
}

# Create A record for production subdomain when ingress external IP is known
resource "azurerm_dns_a_record" "production" {
  count               = var.ingress_external_ip != "" ? 1 : 0
  name                = var.production_subdomain
  zone_name           = azurerm_dns_zone.main[0].name
  resource_group_name = var.resource_group_name
  ttl                 = var.dns_record_ttl
  records             = [var.ingress_external_ip]

  tags = var.tags
}
