resource "aws_secretsmanager_secret" "db_url" {
  name        = "${var.project}/DB_URL"
  description = "PostgreSQL connection string for UMI backend"

  tags = {
    Name        = "${var.project}-secret-db"
    Environment = var.environment
  }
}

resource "aws_secretsmanager_secret_version" "db_url" {
  secret_id = aws_secretsmanager_secret.db_url.id
  secret_string = "postgres://${var.db_username}:${random_password.db.result}@${aws_db_instance.main.address}:5432/${var.db_name}?sslmode=require"
}

resource "aws_secretsmanager_secret" "jwt_secret" {
  name        = "${var.project}/JWT_SECRET"
  description = "JWT signing secret for UMI backend"

  tags = {
    Name        = "${var.project}-secret-jwt"
    Environment = var.environment
  }
}

resource "aws_secretsmanager_secret_version" "jwt_secret" {
  secret_id     = aws_secretsmanager_secret.jwt_secret.id
  secret_string = random_password.jwt.result
}

resource "random_password" "jwt" {
  length  = 64
  special = false
}
