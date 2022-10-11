#!/bin/bash
printf "Please enter Starlight Proxy address (example: \033[92mproxy.mc256.dev:8090\033[0m):"
read proxy_address
printf "Enable HTTPS Certificate (requires load balancer like Nginx) (y/\033[92mN\033[0m):"
read yn_response
if [[ ! $yn_response =~ ^[Yy]$ ]]
then
  # NO
  sed "s/STARLIGHT_PROXY/--plain-http --server=$proxy_address/" demo/deb-package/debian/starlight-snapshotter.service > /lib/systemd/system/starlight.service
else
  # YES
  sed "s/STARLIGHT_PROXY/--server=$proxy_address/" demo/deb-package/debian/starlight-snapshotter.service > /lib/systemd/system/starlight.service
fi
printf "created systemd service file (\033[92m/lib/systemd/system/starlight.service\033[0m) \n"

systemctl daemon-reload

echo "reloaded systemd daemon"