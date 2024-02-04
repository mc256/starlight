output "id" {
  description = "The ec2 instance id"
  value       = aws_instance.ec2.id
  sensitive   = false
}
