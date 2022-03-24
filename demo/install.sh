#!/bin/bash
printf "Please enter Starlight Proxy address (example: \033[92mproxy.mc256.dev:8090\033[0m):"
read proxy_address
printf "Enable HTTPS Certificate (requires load balancer like Nginx) (y/\033[92mN\033[0m):"
read yn_response
echo   # (optional) move to a new line
if [[ ! $yn_response =~ ^[Yy]$ ]]
then
  # NO
  sed "s/STARLIGHT_PROXY/--plain-http $proxy_address/" demo/starlight.service > /lib/systemd/system/starlight.service
else
  # YES
  sed "s/STARLIGHT_PROXY/$proxy_address/" demo/starlight.service > /lib/systemd/system/starlight.service
fi
echo "created systemd service file (\033[92m/lib/systemd/system/starlight.service\033[0m)"
systemctl daemon-reload
echo "reloaded systemd daemon"