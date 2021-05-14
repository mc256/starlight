from common import Runner
from common import ContainerExperimentX as X
from common import MountingPoint as M

if __name__ == '__main__':

    event_suffix = "-dryrun"
    debug = True

    for t in [
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
        print("Hello! This is Starlight Stage. We are running experiment:\n\t- %s" % t)

        t.rounds = 1
        t.update_experiment_name()

        for i in range(len(t.rtt)):
            print("RTT:%d" % t.rtt[i])

            r.service.set_latency_bandwidth(t.rtt[i])  # ADD DELAY

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
