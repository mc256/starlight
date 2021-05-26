from mounting_point import MountingPoint as M
from container_experiment import ContainerExperimentX as X
import subprocess
import os
import time


def redis_ycsb_benchmark(seq: int, method: str, debug: bool = False):
    remote = subprocess.Popen([
        'ssh', 'ubuntu@benchmark',
        'source /home/ubuntu/venv2.7/bin/activate; '
        'export PATH=$PATH:/usr/local/go/bin:/usr/local/apache-maven/bin; '
        'export PATH=$PATH:$JAVA_HOME/bin; '
        'export JAVA_HOME=/usr/lib/jvm/java-8-openjdk-amd64; '
        'cd /home/ubuntu/Development/YCSB; '
        './bin/ycsb run redis -s -P workloads/tsworkloada '
        '-threads 16 '
        '-p "status.interval=100" '
        '-p "redis.host=worker" '
        '-p "redis.port=6379" '
        '-p measurementtype=timeseries '
        '-p timeseries.granularity=100 > /home/ubuntu/sandbox/redis-v8/%d_%s.log  2>/dev/null ' % (
        seq, method)
    ], preexec_fn=os.setpgrp, stdout=subprocess.PIPE, stderr=subprocess.PIPE)

    # takes approximately 20 seconds to start YCSB. We want YCSB to wait for the redis worker
    time.sleep(25)

    return remote


PopBench = {
    # Linux Distribution --------------------------------------------------------------------------
    'alpine': X('alpine', 'distro', '1B', '3.13.5', '3.13.4', [
        M("", overwrite="type=bind,"
                        "src=/home/ubuntu/Development/starlight/demo/config/entrypoint-hello.sh,"
                        "dst=/entrypoint.sh,"
                        "options=rbind:ro"
          )
    ], "hello", ["/entrypoint.sh"], 10),
    'ubuntu': X('ubuntu', 'distro', '1B', 'focal-20210416', 'focal-20210401', [
        M("", overwrite="type=bind,"
                        "src=/home/ubuntu/Development/starlight/demo/config/entrypoint-hello.sh,"
                        "dst=/entrypoint.sh,"
                        "options=rbind:ro"
          )
    ], "hello", ["/entrypoint.sh"], 10),

    # Language ------------------------------------------------------------------------------------
    'node': X('node', 'language', '1B', '16-alpine3.12', '16-alpine3.11', [
        M("", overwrite="type=bind,"
                        "src=/home/ubuntu/Development/starlight/demo/config/entrypoint-js.sh,"
                        "dst=/entrypoint.sh,"
                        "options=rbind:ro"
          ),
        M("", overwrite="type=bind,"
                        "src=/home/ubuntu/Development/starlight/demo/config/scripts,"
                        "dst=/app,"
                        "options=rbind:rw"
          )
    ], "Hello", ["/entrypoint.sh"]),
    'openjdk': X('openjdk', 'language', '1B', '16.0.1-jdk', '11.0.11-9-jdk', [
        M("", overwrite="type=bind,"
                        "src=/home/ubuntu/Development/starlight/demo/config/entrypoint-java.sh,"
                        "dst=/entrypoint.sh,"
                        "options=rbind:ro"
          ),
        M("", overwrite="type=bind,"
                        "src=/home/ubuntu/Development/starlight/demo/config/scripts,"
                        "dst=/app,"
                        "options=rbind:rw"
          )
    ], "Hello", ["/entrypoint.sh"]),
    'golang': X('golang', 'language', '1B', '1.16.3', '1.16.2', [
        M("", overwrite="type=bind,"
                        "src=/home/ubuntu/Development/starlight/demo/config/entrypoint-go.sh,"
                        "dst=/entrypoint.sh,"
                        "options=rbind:ro"
          ),
        M("", overwrite="type=bind,"
                        "src=/home/ubuntu/Development/starlight/demo/config/scripts,"
                        "dst=/app,"
                        "options=rbind:rw"
          )
    ], "Hello", ["/entrypoint.sh"]),
    'python': X('python', 'language', '1B', '3.9.4', '3.9.3', [
        M("", overwrite="type=bind,"
                        "src=/home/ubuntu/Development/starlight/demo/config/entrypoint-py.sh,"
                        "dst=/entrypoint.sh,"
                        "options=rbind:ro"
          ),
        M("", overwrite="type=bind,"
                        "src=/home/ubuntu/Development/starlight/demo/config/scripts,"
                        "dst=/app,"
                        "options=rbind:rw"
          )
    ], "Hello", ["/entrypoint.sh"]),

    # Web Server ------------------------------------------------------------------------------------
    'memcached': X('memcached', 'web-server', '1B', '1.6.9', '1.6.8', [
        M("", overwrite="type=bind,"
                        "src=/home/ubuntu/Development/starlight/demo/config/entrypoint-memcached.sh,"
                        "dst=/entrypoint.sh,"
                        "options=rbind:ro"
          )
    ], "server listening", ["/entrypoint.sh"]),
    'httpd': X('httpd', 'web-server', '1B', '2.4.46', '2.4.43', [], "Command line: 'httpd -D FOREGROUND'"),
    'nginx': X('nginx', 'web-server', '1B', '1.20.0', '1.19.10', [], "ready for start up"),

    # Database --------------------------------------------------------------------------------------
    'mysql': X('mysql', 'database', '1B', '8.0.24', '8.0.23', [
        M("/var/lib/mysql", False, "rw", "999:999"),
        M("/run/mysqld", False, "rw", "999:999")
    ], "port: 3306  MySQL Community Server - GPL",
               None, 40
               ),
    'mariadb': X('mariadb', 'database', '1B', '10.5.9', '10.5.8', [
        M("/var/lib/mysql", False, "rw", "999:999"),
        M("/run/mysqld", False, "rw", "999:999")
    ], "port: 3306  mariadb.org binary distribution",
                 None, 40
                 ),
    'redis': X('redis', 'database', '1B', '6.2.2', '6.2.1',
               [M("/data")],
               "* Ready to accept connections",
               ["/usr/local/bin/redis-server", "--protected-mode", "no"], 10
               ),

    'redis-ycsb': X('redis', 'database', '1B', '6.2.2', '6.2.1',
                    [M("/data"), M("", overwrite="type=bind,"
                                                 "src=/home/ubuntu/Development/starlight/demo/config/entrypoint-redis.sh,"
                                                 "dst=/entrypoint.sh,"
                                                 "options=rbind:ro"
                                   )],
                    "* Ready to accept connections",
                    ["/entrypoint.sh"], 10, redis_ycsb_benchmark
                    ),

    'postgres': X('postgres', 'database', '1B', '13.2', '13.1',
                  [M("/var/lib/postgresql/data")],
                  "LOG:  database system is ready to accept connections",
                  None, 30
                  ),
    'mongo': X('mongo', 'database', '1B', '4.0.24', '4.0.23', [M("/data/db")],
               "waiting for connections on port 27017"
               ),

    # Application --------------------------------------------------------------------------------------
    'rabbitmq': X('rabbitmq', 'application', '1B', '3.8.14', '3.8.13', [], "Server startup complete", None, 30),
    'registry': X('registry', 'application', '1B', '2.7.1', '2.7.0', [M("/data")], "listening on [::]:5000", None, 10),
    'wordpress-fpm': X('wordpress', 'application', '1B', 'php7.4-fpm', 'php7.3-fpm', [
        M("/var/www/html")
    ], "ready to handle connections"),
    'wordpress': X('wordpress', 'application', '1B', 'php7.4', 'php7.3', [
        M("/var/www/html")
    ], "Command line: 'apache2 -D FOREGROUND'"),
    'nextcloud': X('nextcloud', 'application', '1B', '21.0.1-apache', '20.0.9-apache', [],
                   "Command line: 'apache2 -D FOREGROUND'"),
    'nextcloud-fpm': X('nextcloud', 'application', '1B', '21.0.1-fpm', '20.0.9-fpm', [],
                       "ready to handle connections"),
    'ghost': X('ghost', 'application', '1B', '4.3.3-alpine', '3.42.5-alpine', [
        M("/var/lib/ghost/content", False, "rw", "3001:2368")
    ], "Ghost boot"),

    # Edge ------------------------------------------------------------------------------------------------

    'flink': X('flink', 'emerging', '50M', '1.12.3-scala_2.12-java8', '1.12.3-scala_2.11-java8', [],
               "Starting RPC endpoint for org.apache.flink.runtime.dispatcher.StandaloneDispatcher",
               ["/docker-entrypoint.sh", "jobmanager"]),
    'cassandra': X('cassandra', 'emerging', '100M', '3.11.10', '3.11.9',
                   [M("/var/lib/cassandra")],
                   "- Startup complete",
                   None, 30
                   ),
    'eclipse-mosquitto': X('eclipse-mosquitto', 'emerging', '100M', '2.0.10-openssl', '2.0.9-openssl', [
        M("/mosquitto/data"),
        M("/mosquitto/log"),
    ], "running")
}
