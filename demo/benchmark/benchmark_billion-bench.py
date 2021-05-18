from common import MountingPoint as M
from common import ContainerExperimentX as X

BillionBench = [
    X(
        'mysql', 'database', '1B', '8.0.24', '8.0.23', [
            M("/var/lib/mysql", False, "rw", "999:999"),
            M("/run/mysqld", False, "rw", "999:999")
        ], "port: 3306  MySQL Community Server - GPL",
        None, 40
    ),
    X(
        'mariadb', 'database', '1B', '10.5.9', '10.5.8', [
            M("/var/lib/mysql", False, "rw", "999:999"),
            M("/run/mysqld", False, "rw", "999:999")
        ], "port: 3306  mariadb.org binary distribution",
        None, 40
    ),
    X(
        'postgres', 'database', '1B', '13.2', '13.1',
        [M("/var/lib/postgresql/data")],
        "LOG:  database system is ready to accept connections"
    ),
    X(
        'redis', 'database', '1B', '6.2.2', '6.2.1', [M("/data")],
        "* Ready to accept connections"
    ),

    X('rabbitmq', 'application', '1B', '3.8.14', '3.8.13', [], "Server startup complete"),
    X('registry', 'application', '1B', '2.7.1', '2.7.0', [M("/data")], "listening on [::]:5000"),

    X('wordpress', 'application', '1B', 'php7.4-fpm', 'php7.3-fpm', [M("/var/www/html")],
      "ready to handle connections"),
    X('nextcloud', 'application', '1B', '21.0.1-apache', '20.0.9-apache', [M("/var/www/html")],
      "Command line: 'apache2 -D FOREGROUND'"),
    X('ghost', 'application', '1B', '4.3.3-alpine', '3.42.5-alpine',
      [M("/var/lib/ghost/content", False, "rw", "3001:2368")], "Ghost booted"),
    X('phpmyadmin', 'application', '10M', '5.1.0-fpm-alpine', '5.0.4-fpm-alpine', [],
      "NOTICE: ready to handle connections"),

    X('httpd', 'web-server', '1B', '2.4.46', '2.4.43', [], "Command line: 'httpd -D FOREGROUND'"),
    X('nginx', 'web-server', '1B', '1.20.0', '1.19.10', [], "ready for start up"),

    X('flink', 'emerging', '50M', '1.12.3-scala_2.12-java8', '1.12.3-scala_2.11-java8', [],
      "Starting RPC endpoint for org.apache.flink.runtime.dispatcher.StandaloneDispatcher",
      ["/docker-entrypoint.sh", "jobmanager"]),
    X('cassandra', 'emerging', '100M', '3.11.10', '3.11.9', [M("/var/lib/cassandra")],
      "- Startup complete", None, 40),
    X('eclipse-mosquitto', 'emerging', '100M', '2.0.10-openssl', '2.0.9-openssl', [
        M("/mosquitto/data"),
        M("/mosquitto/log"),
    ], "running"),
    X(
        'mongo', 'database', '1B', '4.0.24', '4.0.23', [M("/data/db")],
        "waiting for connections on port 27017"
    ),
]

BillionBench_hold = [
    X('python', 'language', '1B', '3.9.4', '3.9.3', [], "Hello"),
    X('node', 'language', '1B', '16-alpine3.12', '16-alpine3.11', [], "Hello"),
    X('openjdk', 'language', '1B', '16.0.1-jdk', '11.0.11-9-jdk', [], "Hello"),
    X('golang', 'language', '1B', '1.16.3', '1.16.2', [], "Hello"),

    X('haproxy', 'application', '500M', '2.3.10', '2.3.9', [], ""),
    X('traefik', 'application', '1B', 'v2.4.7', 'v2.4.6', [], ""),

    X('memcached', 'web-server', '1B', '1.6.9', '1.6.8', [], "server listening", ["memcached", "-m", "256", "-vv"]),
]

#PgAdmin = pgdb pgAdmin
#sharelatex = mongo redis sharelatex