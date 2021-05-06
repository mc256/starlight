#!/bin/bash

REGISTRY="registry.starlight.yuri.moe"
echo $REGISTRY

declare -a ImageList=(
  "ubuntu:focal-20210416"
  "ubuntu:focal-20210401"
  "alpine:3.13.5"
  "alpine:3.13.4"
  "busybox:1.32.1"
  "busybox:1.33.0"
  "busybox:1.32.1"
  "busybox:1.32.0"
  "debian:oldstable"
  "debian:stable"
  "centos:7"
  "centos:8"
  "fedora:32"
  "fedora:33"
  "oraclelinux:7"
  "oraclelinux:8"
)

for VAL in "${ImageList[@]}"; do
  echo "============================================================"
  echo $VAL
  echo "============================================================"
  ctr-remote image optimize --plain-http --entrypoint='[ "/bin/sh", "-c" ]' --args='[ "echo hello" ]' \
  	"$VAL" "http://$REGISTRY/$VAL-starlight"
done

declare -a DatabaseList=(
  "mysql:8.0.20"
  "mysql:8.0.21"
  "mysql:8.0.22"
  "mysql:8.0.23"
  "mysql:8.0.24"
  "mariadb:10.5.9"
  "mariadb:10.5.8"
)

for VAL in "${DatabaseList[@]}"; do
  echo "============================================================"
  echo "$VAL --- Please press Ctrl+C when finished"
  echo "============================================================"

  mkdir /tmp/t1
  mkdir /tmp/t2
  chown -R 999:999 /tmp/t1
  chown -R 999:999 /tmp/t2

  ctr-remote image optimize --plain-http \
    --env-file=../config/all.env \
    --mount type=bind,src=/tmp/t1,dst=/var/lib/mysql,options=bind:rw \
  	--mount type=bind,src=/tmp/t2,dst=/run/mysqld,options=bind:rw \
	  --wait-on-signal \
	"$VAL" "http://$REGISTRY/$VAL-starlight"

	rm -rf /tmp/t1
	rm -rf /tmp/t2
done


declare -a CassandraList=(
  "cassandra:3.11.10"
  "cassandra:3.11.9"
)

for VAL in "${CassandraList[@]}"; do
  echo "============================================================"
  echo "$VAL --- Please press Ctrl+C when finished"
  echo "============================================================"

  mkdir /tmp/t1
  ctr-remote image optimize --plain-http \
      --env-file=../config/all.env \
      --add-hosts=127.0.0.1:localhost \
      --mount type=bind,src=/etc/hosts,dst=/etc/hosts,options=bind:ro \
      --mount type=bind,src=/tmp/t1,dst=/var/lib/cassandra,options=bind:rw \
      --dns-nameservers=8.8.8.8 \
      --wait-on-signal \
    "$VAL" "http://$REGISTRY/$VAL-starlight"
	rm -rf /tmp/t1
done


declare -a RedisList=(
  "redis:5.0"
  "redis:6.0"
  "redis:6.2"
  "redis:6.2.2"
  "redis:6.2.1"
)
for VAL in "${RedisList[@]}"; do
  echo "============================================================"
  echo "$VAL --- Please press Ctrl+C when finished"
  echo "============================================================"

  mkdir /tmp/t1
  ctr-remote image optimize --plain-http \
      --env-file=../config/all.env \
      --mount type=bind,src=/tmp/t1,dst=/data,options=bind:rw \
      --wait-on-signal \
    "$VAL" "http://$REGISTRY/$VAL-starlight"

	rm -rf /tmp/t1
done


declare -a PGLIST=(
  "postgres:13.2"
  "postgres:13.1"
)

for VAL in "${PGLIST[@]}"; do
  echo "============================================================"
  echo "$VAL --- Please press Ctrl+C when finished"
  echo "============================================================"

  mkdir /tmp/m1

  ctr-remote image optimize --plain-http \
      --env-file=../config/all.env \
      --add-hosts=127.0.0.1:localhost \
      --mount type=bind,src=/etc/hosts,dst=/etc/hosts,options=rbind:ro \
      --mount type=bind,src=/tmp/m1,dst=/var/lib/postgresql/data,options=rbind:rw \
      --dns-nameservers=8.8.8.8 \
      --wait-on-signal \
    "$VAL" "http://$REGISTRY/$VAL-starlight"


  rm -rf /tmp/m1
done


declare -a MongoList=(
  "mongo:4.0.24"
  "mongo:4.0.23"
)
for VAL in "${MongoList[@]}"; do
  echo "============================================================"
  echo "$VAL --- Please press Ctrl+C when finished"
  echo "============================================================"

  mkdir /tmp/t1
  ctr-remote image optimize --plain-http \
      --env-file=../config/all.env \
      --mount type=bind,src=/tmp/t1,dst=/data/db,options=rbind:rw \
      --wait-on-signal \
    "$VAL" "http://$REGISTRY/$VAL-starlight"

	rm -rf /tmp/t1
done

declare -a PythonList=(
  "python:3.9.4"
  "python:3.9.3"
)
for VAL in "${PythonList[@]}"; do
  echo "============================================================"
  echo "$VAL --- Please press Ctrl+C when finished"
  echo "============================================================"

  mkdir /tmp/t1
  ctr-remote image optimize --plain-http \
      --env-file=../config/all.env \
      --mount type=bind,src=/tmp/t1,dst=/data/db,options=rbind:rw \
      --wait-on-signal \
      --entrypoint='[ "python", "-c" ]' --args="[\"print('hello')\"]" \
    "$VAL" "http://$REGISTRY/$VAL-starlight"

	rm -rf /tmp/t1
done

declare -a RabbitMQList=(
  "rabbitmq:3.8.14"
  "rabbitmq:3.8.13"
)
for VAL in "${RabbitMQList[@]}"; do
  echo "============================================================"
  echo "$VAL --- Please press Ctrl+C when finished"
  echo "============================================================"

  mkdir /tmp/t1
  ctr-remote image optimize --plain-http \
      --env-file=../config/all.env \
      --wait-on-signal \
    "$VAL" "http://$REGISTRY/$VAL-starlight"

	rm -rf /tmp/t1
done









# =========================== WORDPRESS ===========================
mkdir /tmp/tm
mkdir /tmp/tn
chown -R 999:999 /tmp/tm
chown -R 999:999 /tmp/tn

ctr i pull docker.io/library/mariadb:10.5
ctr c create \
    --env-file ../config/all.env \
    --net-host \
    --mount type=bind,src=/tmp/tm,dst=/var/lib/mysql,options=rbind:rw \
  	--mount type=bind,src=/tmp/tn,dst=/run/mysqld,options=rbind:rw \
    docker.io/library/mariadb:10.5 mdb
ctr t start mdb


echo "sleep for 30 seconds wait until MariaDB is ready"
sleep 30

declare -a WPLIST=(
  "wordpress:php7.4-fpm"
  "wordpress:php7.3-fpm"
)
mkdir /tmp/t1

for VAL in "${WPLIST[@]}"; do
  echo "============================================================"
  echo "$VAL --- Please press Ctrl+C when finished"
  echo "============================================================"


  ctr-remote image optimize --plain-http \
      --env-file=../config/all.env \
      --mount type=bind,src=/tmp/t1,dst=/var/www/html,options=rbind:rw \
      --wait-on-signal \
      --add-hosts=127.0.0.1:localhost \
      --mount type=bind,src=/etc/hosts,dst=/etc/hosts,options=bind:ro \
      --dns-nameservers=8.8.8.8 \
    "$VAL" "http://$REGISTRY/$VAL-starlight"

done

ctr task kill mdb

rm -rf /tmp/t1
rm -rf /tmp/tm
rm -rf /tmp/tn


# =========================== NextCloud ===========================

declare -a NEXTCLOUD=(
  "nextcloud:21.0.1-apache"
  "nextcloud:20.0.9-apache"
)

for VAL in "${NEXTCLOUD[@]}"; do
  echo "============================================================"
  echo "$VAL --- Please press Ctrl+C when finished"
  echo "============================================================"


  mkdir /tmp/t1
  ctr-remote image optimize --plain-http \
      --env-file=../config/all.env \
      --mount type=bind,src=/tmp/t1,dst=/var/www/html,options=rbind:rw \
      --wait-on-signal \
      --add-hosts=127.0.0.1:localhost \
      --mount type=bind,src=/etc/hosts,dst=/etc/hosts,options=bind:ro \
      --dns-nameservers=8.8.8.8 \
    "$VAL" "http://$REGISTRY/$VAL-starlight"

  rm -rf /tmp/t1
done

# =========================== GHOST ===========================
declare -a GHOST=(
  "ghost:4.3.3-alpine"
  "ghost:3.42.5-alpine"
)

for VAL in "${GHOST[@]}"; do
  echo "============================================================"
  echo "$VAL --- Please press Ctrl+C when finished"
  echo "============================================================"


  mkdir /tmp/t1
  chown -R 3001:2368 /tmp/t1
  ctr-remote image optimize --plain-http \
      --env-file=../config/all.env \
      --mount type=bind,src=/tmp/t1,dst=/var/lib/ghost/content,options=rbind:rw \
      --wait-on-signal \
      --add-hosts=127.0.0.1:localhost \
      --mount type=bind,src=/etc/hosts,dst=/etc/hosts,options=bind:ro \
      --dns-nameservers=8.8.8.8 \
    "$VAL" "http://$REGISTRY/$VAL-starlight"

  rm -rf /tmp/t1

done



# =========================== PHPMYADMIN ===========================

declare -a PHPMYADMIN=(
  "phpmyadmin:5.1.0-fpm-alpine"
  "phpmyadmin:5.0.4-fpm-alpine"
)

for VAL in "${PHPMYADMIN[@]}"; do
  echo "============================================================"
  echo "$VAL --- Please press Ctrl+C when finished"
  echo "============================================================"


  mkdir /tmp/t1
  ctr-remote image optimize --plain-http \
      --env-file=../config/all.env \
      --mount type=bind,src=/tmp/t1,dst=/var/www/html,options=rbind:rw \
      --wait-on-signal \
      --add-hosts=127.0.0.1:localhost \
      --mount type=bind,src=/etc/hosts,dst=/etc/hosts,options=bind:ro \
      --dns-nameservers=8.8.8.8 \
    "$VAL" "http://$REGISTRY/$VAL-starlight"

  rm -rf /tmp/t1
done


# =========================== HTTPD ===========================
declare -a HTTPDLIST=(
  "httpd:2.4.46"
  "httpd:2.4.43"
)

for VAL in "${HTTPDLIST[@]}"; do
  echo "============================================================"
  echo "$VAL --- Please press Ctrl+C when finished"
  echo "============================================================"


  mkdir /tmp/t1
  ctr-remote image optimize --plain-http \
      --env-file=../config/all.env \
      --wait-on-signal \
      --add-hosts=127.0.0.1:localhost \
      --mount type=bind,src=/etc/hosts,dst=/etc/hosts,options=bind:ro \
      --dns-nameservers=8.8.8.8 \
    "$VAL" "http://$REGISTRY/$VAL-starlight"

  rm -rf /tmp/t1
done


# =========================== NGINX ===========================
declare -a NGINXLIST=(
  "nginx:1.19.10"
  "nginx:1.20.0"
)

for VAL in "${NGINXLIST[@]}"; do
  echo "============================================================"
  echo "$VAL --- Please press Ctrl+C when finished"
  echo "============================================================"


  mkdir /tmp/t1
  ctr-remote image optimize --plain-http \
      --env-file=../config/all.env \
      --wait-on-signal \
      --add-hosts=127.0.0.1:localhost \
      --mount type=bind,src=/etc/hosts,dst=/etc/hosts,options=bind:ro \
      --dns-nameservers=8.8.8.8 \
    "$VAL" "http://$REGISTRY/$VAL-starlight"

  rm -rf /tmp/t1
done


# =========================== Flink ===========================
declare -a FLINKLIST=(
  "flink:1.12.3-scala_2.12-java8"
  "flink:1.12.3-scala_2.11-java8"
)

for VAL in "${FLINKLIST[@]}"; do
  echo "============================================================"
  echo "$VAL --- Please press Ctrl+C when finished"
  echo "============================================================"


  ctr-remote image optimize --plain-http \
      --env-file=../config/all.env \
      --wait-on-signal \
      --add-hosts=127.0.0.1:localhost \
      --mount type=bind,src=/etc/hosts,dst=/etc/hosts,options=bind:ro \
      --entrypoint='[ "/docker-entrypoint.sh" ]' --args="[\"jobmanager\"]" \
      --dns-nameservers=8.8.8.8 \
    "$VAL" "http://$REGISTRY/$VAL-starlight"

done



# =========================== MOSQUITTO ===========================
declare -a MOSQUITTOLIST=(
  "eclipse-mosquitto:2.0.10-openssl"
  "eclipse-mosquitto:2.0.9-openssl"
)

for VAL in "${MOSQUITTOLIST[@]}"; do
  echo "============================================================"
  echo "$VAL --- Please press Ctrl+C when finished"
  echo "============================================================"
  mkdir /tmp/t1
  mkdir /tmp/t2
  mkdir /tmp/t3

  ctr-remote image optimize --plain-http \
      --env-file=../config/all.env \
      --wait-on-signal \
      --add-hosts=127.0.0.1:localhost \
      --mount type=bind,src=/etc/hosts,dst=/etc/hosts,options=bind:ro \
      --mount type=bind,src=/tmp/t2,dst=/mosquitto/data,options=rbind:rw \
      --mount type=bind,src=/tmp/t3,dst=/mosquitto/log,options=rbind:rw \
      --dns-nameservers=8.8.8.8 \
    "$VAL" "http://$REGISTRY/$VAL-starlight"

  rm -rf /tmp/t1
  rm -rf /tmp/t2
  rm -rf /tmp/t3

done
