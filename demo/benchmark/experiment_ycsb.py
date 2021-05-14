from common import Runner
import os
from common import ContainerExperimentX as X
from common import MountingPoint as M

if __name__ == '__main__':
    experiment_list = [
        X(
            'redis', 'database', '1B', '6.2.2', '6.2.1',
            [M("/data")],
            "* Ready to accept connections",
            None, 10
        )
    ]

    for t in experiment_list:
        r = Runner()
        discard = []

        t.rounds = 10
        t.update_experiment_name()

        rtt = 150
        debug = True

        r.service.reset_latency_bandwidth(True)
        r.service.set_latency_bandwidth(rtt)

        print("RTT:%d" % rtt)

        print("Hello! This is Starlight Stage. We are running experiment:\n\t- %s" % t)

        r.ycsb_base = "150ms-tsworkloada-m5a.2x-1thread"
        event_suffix = "-ycsb-%s" % r.ycsb_base
        r.ycsb_base = r.ycsb_base + "/"
        try:
            os.mkdir("%s/%s" % (r.service.config.YCSB_LOG, r.ycsb_base))
        except:
            pass

        # estargz
        for k in range(t.rounds + 1):
            r.service.reset_container_service()
            r.service.start_grpc_estargz()

            n = 0
            if t.has_old_version():
                n = r.test_estargz(
                    t,
                    k == 0, rtt=rtt, seq=k,
                    use_old=True,
                    r=n,
                    debug=False,
                    ycsb=False if k == 0 else True
                )
                pass

            r.test_estargz(
                t,
                k == 0, rtt=rtt, seq=k,
                use_old=False,
                r=n,
                debug=False,
                ycsb=False if k == 0 else True
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
                    k == 0, rtt=rtt, seq=k,
                    use_old=True,
                    r=n,
                    debug=False,
                    ycsb=False if k == 0 else True
                )
                pass

            r.test_starlight(
                t,
                k == 0, rtt=rtt, seq=k,
                use_old=False,
                r=n,
                debug=False,
                ycsb=False if k == 0 else True
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
                    k == 0, rtt=rtt, seq=k,
                    use_old=True,
                    r=n,
                    debug=False,
                    ycsb=False if k == 0 else True
                )
                pass

            r.test_vanilla(
                t,
                k == 0, rtt=rtt, seq=k,
                use_old=False,
                r=n,
                debug=False,
                ycsb=False if k == 0 else True
            )
            t.save_event(event_suffix)

        """
        # wget
        for k in range(t.rounds + 1):
            n = r.test_wget(t, k == 0, rtt=rtt, seq=k, r=0, use_old=True)
            r.test_wget(t, k == 0, rtt=rtt, seq=k, r=n, use_old=False)
        """

        # save results
        t.save_event(event_suffix)
        r.service.reset_container_service()
