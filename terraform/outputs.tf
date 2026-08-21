output "alb_dns" {
  description = "Backend API URL (HTTP)"
  value       = "http://${aws_lb.main.dns_name}"
}

output "admin_url" {
  description = "Admin panel URL (CloudFront)"
  value       = "https://${aws_cloudfront_distribution.admin.domain_name}"
}

output "ecr_repository" {
  description = "ECR repository URL"
  value       = aws_ecr_repository.main.repository_url
}

output "db_secret_arn" {
  description = "Secrets Manager ARN for DB_URL"
  value       = aws_secretsmanager_secret.db_url.arn
}
