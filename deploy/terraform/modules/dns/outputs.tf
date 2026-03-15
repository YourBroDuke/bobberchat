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
