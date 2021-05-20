from common import Runner
from common import ContainerExperimentX as X
from common import MountingPoint as M
from benchmark_pop_bench import PopBench

"""
            M("", overwrite="type=bind,"
                        "src=/home/ubuntu/Development/starlight/demo/config/my.cnf,"
                        "dst=/etc/my.cnf,"
                        "options=rbind:ro"
            )
            X('memcached', 'web-server', '1B', '1.6.9', '1.6.8', [M("", overwrite="type=bind,"
                        "src=/home/ubuntu/Development/starlight/demo/config/entrypoint-memcached.sh,"
                        "dst=/entrypoint.sh,"
                        "options=rbind:ro"
            )], "server listening", ["/entrypoint.sh"])
            


        X('ghost', 'application', '1B', '4.3.3-alpine', '3.42.5-alpine',
      [M("/var/lib/ghost/content", False, "rw", "3001:2368")], "Ghost boot"),
        X(
            'mysql', 'database', '1B', '8.0.24', '8.0.23', [
                M("/var/lib/mysql", False, "rw", "999:999"),
                M("/run/mysqld", False, "rw", "999:999")
            ], "port: 3306  MySQL Community Server - GPL",
            None, 40
        ),
"""
if __name__ == '__main__':

    event_suffix = "-dryrun"
    debug = True

    for key in ['nextcloud']:
        t = PopBench[key]
        r = Runner()
        discard = []

        r.service.reset_latency_bandwidth()

        t.rounds = 1
        t.rtt = [150]
        t.update_experiment_name()

        print("Hello! This is Starlight Stage. We are running experiment:\n\t- %s" % t)

        for i in range(len(t.rtt)):
            print("RTT:%d" % t.rtt[i])

            r.service.set_latency_bandwidth(t.rtt[i])  # ADD DELAY

            # starlight
            for k in range(t.rounds):

                r.service.reset_container_service()
                r.service.start_grpc_starlight()

                n = 0
                if t.has_old_version():
                    n = r.test_starlight(
                        t,
                        False, rtt=t.rtt[i], seq=k,
                        use_old=True,
                        r=n,
                        debug=debug,
                        ycsb=False
                    )
                    pass

                r.test_starlight(
                    t,
                    False, rtt=t.rtt[i], seq=k,
                    use_old=False,
                    r=n,
                    debug=debug,
                    ycsb=False
                )

                r.service.kill_starlight()
                t.save_event(event_suffix)


            # estargz
            for k in range(t.rounds):
                r.service.reset_container_service()
                r.service.start_grpc_estargz()

                n = 0
                if t.has_old_version():
                    n = r.test_estargz(
                        t,
                        False, rtt=t.rtt[i], seq=k,
                        use_old=True,
                        r=n,
                        debug=debug,
                        ycsb=False
                    )
                    pass

                r.test_estargz(
                    t,
                    False, rtt=t.rtt[i], seq=k,
                    use_old=False,
                    r=n,
                    debug=debug,
                    ycsb=False
                )

                r.service.kill_estargz()
                t.save_event(event_suffix)



            # vanilla
            for k in range(t.rounds):
                r.service.reset_container_service()

                n = 0
                if t.has_old_version():
                    n = r.test_vanilla(
                        t,
                        False, rtt=t.rtt[i], seq=k,
                        use_old=True,
                        r=n,
                        debug=debug,
                        ycsb=False
                    )
                    pass

                r.test_vanilla(
                    t,
                    False, rtt=t.rtt[i], seq=k,
                    use_old=False,
                    r=n,
                    debug=debug,
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
