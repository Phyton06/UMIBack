output "alb_dns" {
  description = "ALB public DNS name"
  value       = aws_lb.main.dns_name
}

output "api_url" {
  description = "Backend API URL"
  value       = "https://api.${var.domain}"
}

output "admin_url" {
  description = "Admin panel URL"
  value       = "https://admin.${var.domain}"
}

output "ecr_repository" {
  description = "ECR repository URL"
  value       = aws_ecr_repository.main.repository_url
}

output "rds_endpoint" {
  description = "RDS endpoint (reference via Secrets Manager in production)"
  value       = aws_db_instance.main.address
}

output "db_secret_arn" {
  description = "Secrets Manager ARN for DB_URL"
  value       = aws_secretsmanager_secret.db_url.arn
}

output "jwt_secret_arn" {
  description = "Secrets Manager ARN for JWT_SECRET"
  value       = aws_secretsmanager_secret.jwt_secret.arn
}
