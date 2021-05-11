from common import Runner
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

        pool_starlight = []
        pool_vanilla = []
        pool_estargz = []
        pool_wget = []

        pool_starlight_update = []
        pool_vanilla_update = []
        pool_estargz_update = []
        pool_wget_update = []

        step_starlight = []
        step_vanilla = []
        step_estargz = []
        step_wget = []

        step_starlight_update = []
        step_vanilla_update = []
        step_estargz_update = []
        step_wget_update = []

        # estargz
        for k in range(t.rounds + 1):
            r.service.reset_container_service()
            r.service.start_grpc_estargz()

            n = 0
            if t.has_old_version():
                n = r.test_estargz(t, history=discard if k == 0 else step_estargz, use_old=True, r=n, debug=False,
                                   ycsb=False if k == 0 else True)
                r.service.set_latency_bandwidth(rtt)
                pass

            r.test_estargz(t, history=discard if k == 0 else step_estargz_update, use_old=False, r=n, debug=False,
                           ycsb=False if k == 0 else True)

            r.service.kill_estargz()

        # starlight
        for k in range(t.rounds + 1):
            r.service.reset_container_service()
            r.service.start_grpc_starlight()

            n = 0
            if t.has_old_version():
                n = r.test_starlight(t, history=discard if k == 0 else step_starlight, use_old=True, r=n, debug=False,
                                     ycsb=False if k == 0 else True)
                r.service.set_latency_bandwidth(rtt)
                pass

            r.test_starlight(t, history=discard if k == 0 else step_starlight_update, use_old=False, r=n, debug=False,
                             ycsb=False if k == 0 else True)

            r.service.kill_starlight()

        # vanilla
        for k in range(t.rounds + 1):
            r.service.reset_container_service()

            n = 0
            if t.has_old_version():
                n = r.test_vanilla(t, history=discard if k == 0 else step_vanilla, use_old=True, r=n, debug=False,
                                   ycsb=False if k == 0 else True)
            r.test_vanilla(t, history=discard if k == 0 else step_vanilla_update, use_old=False, r=n, debug=False,
                           ycsb=False if k == 0 else True)

        # wget
        for k in range(t.rounds + 1):
            r.test_wget(t, history=discard if k == 0 else step_wget, use_old=True)
            r.test_wget(t, history=discard if k == 0 else step_wget_update, use_old=False)

        # save results
        pool_starlight.append(step_starlight)
        pool_vanilla.append(step_vanilla)
        pool_estargz.append(step_estargz)
        pool_wget.append(step_wget)

        _, _, _, _ = t.save_results(
            pool_estargz, pool_starlight, pool_vanilla, pool_wget, 1, "-deploy-fixed"
        )

        pool_starlight_update.append(step_starlight_update)
        pool_vanilla_update.append(step_vanilla_update)
        pool_estargz_update.append(step_estargz_update)
        pool_wget_update.append(step_wget_update)

        _, _, _, _ = t.save_results(
            pool_estargz_update, pool_starlight_update, pool_vanilla_update, pool_wget_update, 1, "-update-fixed"
        )

        # plotting
        t.plot_single_result(
            step_starlight, step_vanilla, step_estargz, step_wget, "-deploy-fixed"
        )
        t.plot_single_result(
            step_starlight_update, step_vanilla_update, step_estargz_update, step_wget_update, "-update-fixed"
        )

        r.service.reset_container_service()
