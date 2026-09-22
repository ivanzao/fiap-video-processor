resource "aws_s3_bucket" "this" {
  count = var.manage_bucket ? 1 : 0

  bucket        = var.bucket_name
  force_destroy = var.force_destroy
}

resource "aws_s3_bucket_public_access_block" "this" {
  bucket = var.bucket_name

  depends_on = [aws_s3_bucket.this]

  block_public_acls       = true
  block_public_policy     = true
  ignore_public_acls      = true
  restrict_public_buckets = true
}

resource "aws_s3_bucket_server_side_encryption_configuration" "this" {
  bucket = var.bucket_name

  depends_on = [aws_s3_bucket.this]

  rule {
    apply_server_side_encryption_by_default {
      sse_algorithm = "AES256"
    }
  }
}

resource "aws_s3_bucket_cors_configuration" "this" {
  bucket = var.bucket_name

  depends_on = [aws_s3_bucket.this]

  cors_rule {
    allowed_origins = var.cors_allowed_origins
    allowed_methods = ["PUT", "GET", "HEAD"]
    allowed_headers = ["*"]
    expose_headers  = ["ETag"]
    max_age_seconds = 3000
  }
}

resource "aws_s3_bucket_lifecycle_configuration" "this" {
  bucket = var.bucket_name

  depends_on = [aws_s3_bucket.this]

  dynamic "rule" {
    for_each = var.expiration_rules
    content {
      id     = "expire-${trimsuffix(replace(rule.key, "/", "-"), "-")}"
      status = "Enabled"

      filter {
        prefix = rule.key
      }

      expiration {
        days = rule.value
      }
    }
  }
}
