output "dns_zone_name" {
  description = "Name of the created DNS zone"
  value       = var.create_dns_zone ? azurerm_dns_zone.main[0].name : ""
}

output "dns_zone_id" {
  description = "ID of the created DNS zone"
  value       = var.create_dns_zone ? azurerm_dns_zone.main[0].id : ""
}

output "name_servers" {
  description = "Name servers for the DNS zone - must be configured at domain registrar for delegation"
  value       = var.create_dns_zone ? azurerm_dns_zone.main[0].name_servers : []
}

output "staging_fqdn" {
  description = "Full Qualified Domain Name of the staging subdomain A record (empty until ingress IP is provided)"
  value       = length(azurerm_dns_a_record.staging) > 0 ? "${var.staging_subdomain}.${var.domain_name}" : ""
}

output "production_fqdn" {
  description = "Full Qualified Domain Name of the production subdomain A record (empty until ingress IP is provided)"
  value       = length(azurerm_dns_a_record.production) > 0 ? "${var.production_subdomain}.${var.domain_name}" : ""
}

output "ingress_external_ip" {
  description = "The external IP address of the nginx-ingress LoadBalancer"
  value       = var.ingress_external_ip
}
