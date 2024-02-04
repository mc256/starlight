output "cloud-instance-id" {
  description = "The ec2 instance id"
  value       = aws_instance.starlight_cloud.id
  sensitive   = false
}

output "cloud-instance-public-ip" {
  description = "The ec2 instance public ip"
  value       = aws_instance.starlight_cloud.public_ip
  sensitive   = false
}

output "edge-instance-id" {
  description = "The ec2 instance id"
  value       = aws_instance.starlight_edge.id
  sensitive   = false
}

output "edge-instance-public-ip" {
  description = "The ec2 instance public ip"
  value       = aws_instance.starlight_edge.public_ip
  sensitive   = false
}
