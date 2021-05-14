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
X('ubuntu', 'distro', '1B', 'focal-20210416', 'focal-20210401', [
    M("", overwrite="type=bind,"
                    "src=/home/ubuntu/Development/starlight/demo/config/hello-entrypoint.sh,"
                    "dst=/entrypoint.sh,"
                    "options=rbind:ro"
      )
], "hello", ["/entrypoint.sh"], 10)
"""
if __name__ == '__main__':

    event_suffix = "-v3"

    for t in [
        X(
            'mysql', 'database', '1B', '8.0.24', '8.0.23', [
                M("/var/lib/mysql", False, "rw", "999:999"),
                M("/run/mysqld", False, "rw", "999:999")
            ], "port: 3306  MySQL Community Server - GPL",
            None, 40
        ),
        X('registry', 'application', '1B', '2.7.1', '2.7.0', [M("/data")], "listening on [::]:5000", None, 10),
    ]:

        r = Runner()
        discard = []

        r.service.reset_latency_bandwidth()
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
            for k in range(t.rounds):
                r.test_wget(t, k == 0, rtt=t.rtt[i], seq=k, use_old=True)
                r.test_wget(t, k == 0, rtt=t.rtt[i], seq=k, use_old=False)
                t.save_event(event_suffix)

            r.service.reset_latency_bandwidth()

        r.service.reset_container_service()
        r.service.reset_latency_bandwidth()
