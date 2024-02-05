# required
ssh_key_name = "starlight-key"

# please replace with your own public key
# this is the key for accessing the EC2 instances, if empty, we assume the key above is already created
ssh_public_key = "ssh-rsa AAAAB3NzaC1yc2EAAAABJQAAAgEA08XpxYFsakw1hVCcFYTVvXMF+sxyF9PQ16dF+D1xP3Vr5MGq275FIyKQs0oQxKCCBtUGMIgCVrQ+V3CVu9CsfBTHoJlk1nW0dfJcB3j9kG3+3NRIHZ+FTrIO5hvMWJ+SzXTIDYgdcKLJ/YIboqBI4Rm/rNx7fVIib0K+43RcD4YRbwcMXK1lWkZ2czj+ixKmX5P0EeTLNz2J6XEsLT3V39B4QoXLKJM3TTjlRx2w980UI2lUED+3aW8jeW6Y9v9TUW5ZiZHxFLULQGD4UoivxLHUZCVJbnSaBX5r1yiE2jOvlhCXgaEa/EyYt6gBacMMC/qdIIX6nsts5+K4i/+37Jixd1qyJ5tWcxY9tUQZeiD1kuL0qy/mKtR9ONZLOUFvmWXG7t5i9axVaIyjj+Yb/4PXvEVd3jZ0jC8gcPjkkjOV22CLCCLgwtUHcDTE/OM8/oUem4Tnd/9blBjd47RfuHTdyVsujLwue5hpUo64E3JDSdVee0s1Yso1Wx8ZEfhJgqfAc+E5gSprY2pdFZUwffN52I8+72OfNnsmw1h10LqvN5Xpu+12eARr/LQHY/0E45kJRAcFbAgjUwKmKtdf0UAepb/AwLF37I9UT4uLxs56dvw4z1rJQDGQJdm8GVv0CJ8pzjBsYFYzf43Mp4+lYJ+V7BBMMo5YLa6em/bqi5s= mc256"

# recommended to change to machine with more memory
#
# Current setting is tide to AWS free tier limit 750hours of t3.micro (1GB memory).
cloud_instance_type = "t3.micro"
edge_instance_type  = "t3.micro"


# EBS volume size in GB
# Cloud will need more space for storing the container image and metadata than the edge.
# Please adjust the size according to your needs.
#
# Current setting is tide to AWS EBS free tier limit 30GB
cloud_ebs_size_in_gb = 20
edge_ebs_size_in_gb  = 10
