#!/bin/bash

ENV_FILE='/home/maverick/Development/starlight/demo/config/all.env'

sudo ctr-starlight -n n1 --log-level debug prepare \
mysql:8.0.23-starlight,wordpress:php7.3-fpm-starlight,phpmyadmin:5.0.4-fpm-alpine-starlight && \
sudo ctr-starlight -n n1 --log-level debug create --tty --net-host --local-time \
  --mount type=bind,src=/tmp/benchmark-folders/m1,dst=/var/lib/mysql,options=rbind:rw \
  --mount type=bind,src=/tmp/benchmark-folders/m2,dst=/var/run/mysqld,options=rbind:rw \
  --env-file "$ENV_FILE" \
  mysql:8.0.23-starlight,wordpress:php7.3-fpm-starlight,phpmyadmin:5.0.4-fpm-alpine-starlight \
  mysql:8.0.23-starlight \
  task-a-1 && \
sudo ctr -n n1 t start task-a-1 -d && \
sudo ctr-starlight -n n1 --log-level debug create --tty --net-host --privileged --local-time \
  --mount type=bind,src=/tmp/benchmark-folders/m3,dst=/var/www/html,options=rbind:rw \
  --mount type=bind,src=/etc/hosts,dst=/etc/hosts,options=bind:ro \
  --mount type=bind,src=/etc/resolv.conf,dst=/etc/resolv.conf,options=bind:ro \
  --env-file "$ENV_FILE" \
  mysql:8.0.23-starlight,wordpress:php7.3-fpm-starlight,phpmyadmin:5.0.4-fpm-alpine-starlight \
  wordpress:php7.3-fpm-starlight \
  task-b-2 && \
sudo ctr -n n1 t start task-b-2 && \
sudo ctr-starlight -n n1 --log-level debug create --tty --net-host --local-time \
  --env-file "$ENV_FILE" \
  mysql:8.0.23-starlight,wordpress:php7.3-fpm-starlight,phpmyadmin:5.0.4-fpm-alpine-starlight \
  phpmyadmin:5.0.4-fpm-alpine-starlight \
  task-c-1 && \
sudo ctr -n n1 t start task-c-1


#sudo ctr -n n1 t kill task-a-1 && \
#sudo ctr -n n1 t kill task-b-1 && \
#sudo ctr -n n1 t kill task-c-1