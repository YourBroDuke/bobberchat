output "server_fqdn" {
  description = "Fully qualified domain name of the PostgreSQL Flexible Server"
  value       = azurerm_postgresql_flexible_server.main.fqdn
}

output "server_id" {
  description = "ID of the PostgreSQL Flexible Server"
  value       = azurerm_postgresql_flexible_server.main.id
}

output "database_name" {
  description = "Name of the created database"
  value       = var.database_name
}

output "connection_dsn" {
  description = "PostgreSQL connection string (DSN) in the format: postgres://user:pass@host:5432/dbname?sslmode=require"
  value       = "postgres://${var.administrator_login}:${var.administrator_password}@${azurerm_postgresql_flexible_server.main.fqdn}:5432/${var.database_name}?sslmode=require"
  sensitive   = true
}

output "private_dns_zone_id" {
  description = "ID of the private DNS zone"
  value       = azurerm_private_dns_zone.postgres.id
}
