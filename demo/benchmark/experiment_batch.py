from common import Runner
from common import ContainerExperimentX as X
from common import MountingPoint as M
from benchmark_pop_bench import PopBench

if __name__ == '__main__':

    event_suffix = "-v6"

    for key in ['mysql']:
        t = PopBench[key]
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

            """
            """
            r.service.reset_latency_bandwidth()

        r.service.reset_container_service()
        r.service.reset_latency_bandwidth()
