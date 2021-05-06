# How to install Starlight Proxy

- Install Docker, Docker Compose, Nginx, Certbot (If you want to use Let'sEncrypt)
- Setup Nginx
- Generate SSL certificate
- Apply `sysctl.conf` for a larger TCP window size if needed
- Update the registry information in `config.env` (change the IP for `registry.starlight.yuri.moe`  in `/etc/hosts` if needed)
- Use Docker Compose to launch the starlight proxy 