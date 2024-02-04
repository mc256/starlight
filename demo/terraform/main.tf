## EC2 Image
data "aws_ami" "ubuntu" {
  most_recent = true
  owners      = ["099720109477"] # Canonical

  filter {
    name   = "name"
    values = ["ubuntu/images/hvm-ssd/ubuntu-jammy-22.04-amd64-server-*"]
  }

  filter {
    name   = "virtualization-type"
    values = ["hvm"]
  }

  filter {
    name   = "root-device-type"
    values = ["ebs"]
  }

  filter {
    name   = "architecture"
    values = ["x86_64"]
  }

}

## create vpc
resource "aws_vpc" "ec2_vpc" {
  cidr_block = "10.0.0.0/16"
  tags = merge(
    var.default_tags,
    {
      Name = "${local.project_name}-ec2-vpc"
    },
  )
}

resource "aws_subnet" "ec2_subnet" {
  vpc_id     = aws_vpc.ec2_vpc.id
  cidr_block = "10.0.1.0/24"
  tags = merge(
    var.default_tags,
    {
      Name = "${local.project_name}-ec2-subnet"
    },
  )
}

## Security Group
resource "aws_security_group" "ec2_security_group" {
  name        = "${local.project_name}-ec2-sg"
  description = "Allow inbound traffic from port 8080 and all outbound traffic"
  vpc_id      = aws_vpc.ec2_vpc.id

  tags = merge(
    var.default_tags,
    {
      Name = "${local.project_name}-ec2-sg"
    },
  )
}

resource "aws_vpc_security_group_egress_rule" "all_egress" {
  security_group_id = aws_security_group.ec2_security_group.id

  cidr_ipv4   = "0.0.0.0/0" # all
  ip_protocol = "-1"        # all

}

resource "aws_vpc_security_group_ingress_rule" "http_ingress" {
  security_group_id = aws_security_group.ec2_security_group.id

  cidr_ipv4   = "0.0.0.0/0"
  from_port   = 8080
  to_port     = 8080
  ip_protocol = "tcp"

}


## EC2 Host
resource "aws_instance" "ec2" {
  ami                    = data.aws_ami.ubuntu.id
  instance_type          = var.instance_type
  subnet_id              = aws_subnet.ec2_subnet.id
  key_name               = var.ssh_keyname
  vpc_security_group_ids = [aws_security_group.ec2_security_group.id]
  monitoring             = true

  lifecycle {
    ignore_changes = [subnet_id, ami]
  }

  root_block_device {
    volume_type           = "gp3"
    volume_size           = var.ebs_size_in_gb
    encrypted             = false
    delete_on_termination = true
  }


  tags = merge(
    var.default_tags,
    {
      Name = "${local.project_name}-ec2-cloud"
    },
  )
}
