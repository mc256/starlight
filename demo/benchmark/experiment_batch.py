from common import Runner
from common import ContainerExperimentX as X
from common import MountingPoint as M

"""
    X(
        'redis', 'database', '1B', '6.2.2', '6.2.1',
        [M("/data")],
        "* Ready to accept connections",
        ["/usr/local/bin/redis-server","--protected-mode","no"], 10
    ),
    X(
        'cassandra', 'emerging', '100M', '3.11.10', '3.11.9',
        [M("/var/lib/cassandra")],
        "- Startup complete",
        None, 30
    ),
    X(
        'postgres', 'database', '1B', '13.2', '13.1',
        [M("/var/lib/postgresql/data")],
        "LOG:  database system is ready to accept connections",
        None, 30
    ),
    X('rabbitmq', 'application', '1B', '3.8.14', '3.8.13', [], "Server startup complete", None, 30),
    X('registry', 'application', '1B', '2.7.1', '2.7.0', [M("/data")], "listening on [::]:5000", None, 10),
    X('httpd', 'web-server', '1B', '2.4.46', '2.4.43', [], "Command line: 'httpd -D FOREGROUND'"),
    X('ubuntu', 'distro', '1B', 'focal-20210416', 'focal-20210401', [
        M("", overwrite="type=bind,"
                        "src=/home/ubuntu/Development/starlight/demo/config/entrypoint-hello.sh,"
                        "dst=/entrypoint.sh,"
                        "options=rbind:ro"
          )
    ], "hello", ["/entrypoint.sh"], 10)
    X('alpine', 'distro', '1B', '3.13.5', '3.13.4', [
        M("", overwrite="type=bind,"
                        "src=/home/ubuntu/Development/starlight/demo/config/entrypoint-hello.sh,"
                        "dst=/entrypoint.sh,"
                        "options=rbind:ro"
          )
    ], "hello", ["/entrypoint.sh"], 10)
    X('nginx', 'web-server', '1B', '1.20.0', '1.19.10', [], "ready for start up"),
    X('flink', 'emerging', '50M', '1.12.3-scala_2.12-java8', '1.12.3-scala_2.11-java8', [],
      "Starting RPC endpoint for org.apache.flink.runtime.dispatcher.StandaloneDispatcher",
      ["/docker-entrypoint.sh", "jobmanager"]),
    X('wordpress', 'application', '1B', 'php7.4-fpm', 'php7.3-fpm', [M("/var/www/html")],
      "ready to handle connections"),
    X('ghost', 'application', '1B', '4.3.3-alpine', '3.42.5-alpine',
      [M("/var/lib/ghost/content", False, "rw", "3001:2368")], "Ghost boot"),
    X('phpmyadmin', 'application', '10M', '5.1.0-fpm-alpine', '5.0.4-fpm-alpine', [],
      "NOTICE: ready to handle connections"),
      
        X(
            'mysql', 'database', '1B', '8.0.24', '8.0.23', [
                M("/var/lib/mysql", False, "rw", "999:999"),
                M("/run/mysqld", False, "rw", "999:999")
            ], "port: 3306  MySQL Community Server - GPL",
            None, 40
        ),
        
        
    X('ubuntu', 'distro', '1B', 'focal-20210416', 'focal-20210401', [
        M("", overwrite="type=bind,"
                        "src=/home/ubuntu/Development/starlight/demo/config/entrypoint-hello.sh,"
                        "dst=/entrypoint.sh,"
                        "options=rbind:ro"
          )
    ], "hello", ["/entrypoint.sh"], 10),
    X('registry', 'application', '1B', '2.7.1', '2.7.0', [M("/data")], "listening on [::]:5000", None, 10)
    X(
        'mariadb', 'database', '1B', '10.5.9', '10.5.8', [
            M("/var/lib/mysql", False, "rw", "999:999"),
            M("/run/mysqld", False, "rw", "999:999")
        ], "port: 3306  mariadb.org binary distribution",
        None, 40
    ),
    X(
            'mongo', 'database', '1B', '4.0.24', '4.0.23', [M("/data/db")],
            "waiting for connections on port 27017"
       ),
       
       
        X('python', 'language', '1B', '3.9.4', '3.9.3', [
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
        X('node', 'language', '1B', '16-alpine3.12', '16-alpine3.11', [
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
        X('openjdk', 'language', '1B', '16.0.1-jdk', '11.0.11-9-jdk', [
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
        X('golang', 'language', '1B', '1.16.3', '1.16.2', [
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
        X('memcached', 'web-server', '1B', '1.6.9', '1.6.8', [
            M("", overwrite="type=bind,"
                            "src=/home/ubuntu/Development/starlight/demo/config/entrypoint-memcached.sh,"
                            "dst=/entrypoint.sh,"
                            "options=rbind:ro"
              )
        ], "server listening", ["/entrypoint.sh"]),
        X(
            'mongo', 'database', '1B', '4.0.24', '4.0.23', [M("/data/db")],
            "waiting for connections on port 27017"
        ),
"""

if __name__ == '__main__':

    event_suffix = "-v6"

    for t in [
        X('golang', 'language', '1B', '1.16.3', '1.16.2', [
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
        X(
            'mysql', 'database', '1B', '8.0.24', '8.0.23', [
                M("/var/lib/mysql", False, "rw", "999:999"),
                M("/run/mysqld", False, "rw", "999:999")
            ], "port: 3306  MySQL Community Server - GPL",
            None, 40
        ),
    ]:

        r = Runner()
        discard = []

        r.service.reset_latency_bandwidth()
        # t.rtt = [2]
        t.rounds = 5
        t.update_experiment_name()

        print("Hello! This is Starlight Stage. We are running experiment:\n\t- %s" % t)

        for i in range(len(t.rtt)):
            print("RTT:%d" % t.rtt[i])

            r.service.set_latency_bandwidth(t.rtt[i])  # ADD DELAY

            # estargz
            for k in range(t.rounds + 1):
                r.service.reset_container_service()
                r.service.start_grpc_estargz()

                n = 0
                if t.has_old_version():
                    n = r.test_estargz(
                        t,
                        k == 0, rtt=t.rtt[i], seq=k,
                        use_old=True,
                        r=n,
                        debug=False,
                        ycsb=False
                    )
                    pass

                r.test_estargz(
                    t,
                    k == 0, rtt=t.rtt[i], seq=k,
                    use_old=False,
                    r=n,
                    debug=False,
                    ycsb=False
                )

                r.service.kill_estargz()
                t.save_event(event_suffix)

            # starlight
            for k in range(t.rounds + 1):
                r.service.reset_container_service()
                r.service.start_grpc_starlight()

                n = 0
                if t.has_old_version():
                    n = r.test_starlight(
                        t,
                        k == 0, rtt=t.rtt[i], seq=k,
                        use_old=True,
                        r=n,
                        debug=False,
                        ycsb=False
                    )
                    pass

                r.test_starlight(
                    t,
                    k == 0, rtt=t.rtt[i], seq=k,
                    use_old=False,
                    r=n,
                    debug=False,
                    ycsb=False
                )

                r.service.kill_starlight()
                t.save_event(event_suffix)

            # vanilla
            for k in range(t.rounds + 1):
                r.service.reset_container_service()

                n = 0
                if t.has_old_version():
                    n = r.test_vanilla(
                        t,
                        k == 0, rtt=t.rtt[i], seq=k,
                        use_old=True,
                        r=n,
                        debug=False,
                        ycsb=False
                    )
                    pass

                r.test_vanilla(
                    t,
                    k == 0, rtt=t.rtt[i], seq=k,
                    use_old=False,
                    r=n,
                    debug=False,
                    ycsb=False
                )
                t.save_event(event_suffix)

            # wget
            for k in range(t.rounds + 1):
                r.test_wget(t, k == 0, rtt=t.rtt[i], seq=k, use_old=True)
                r.test_wget(t, k == 0, rtt=t.rtt[i], seq=k, use_old=False)
                t.save_event(event_suffix)

            r.service.reset_latency_bandwidth()

        r.service.reset_container_service()
        r.service.reset_latency_bandwidth()
