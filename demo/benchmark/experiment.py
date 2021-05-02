from common import Runner
from test_cases import *

if __name__ == '__main__':
    # t = TestRedis("6.0", "5.0")
    t = TestCassandra("4.0", "3.11")
    # t = TestMySQL("8.0.24", "8.0.23")
    # t = TestMariadb("10.5", "10.4")

    r = Runner()
    discard = []

    r.service.reset_latency_bandwidth()
    print("Hello! This is Starlight Stage. We are running experiment:\n\t- %s" % t)

    pool_starlight = []
    pool_vanilla = []
    pool_estargz = []
    pool_wget = []

    pool_starlight_update = []
    pool_vanilla_update = []
    pool_estargz_update = []
    pool_wget_update = []

    for i in range(len(t.rtt)):
        print("RTT:%d" % t.rtt[i])

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

            r.service.set_latency_bandwidth(t.rtt[i])  # ADD DELAY

            n = 0
            if t.has_old_version():
                n = r.test_estargz(t, history=discard if k == 0 else step_estargz, use_old=True, r=n, debug=False)
            r.test_estargz(t, history=discard if k == 0 else step_estargz_update, use_old=False, r=n, debug=False)

            r.service.reset_latency_bandwidth()  # REMOVE DELAY

            r.service.kill_estargz()

        # starlight
        for k in range(t.rounds + 1):
            r.service.reset_container_service()
            r.service.start_grpc_starlight()

            r.service.set_latency_bandwidth(t.rtt[i])  # ADD DELAY

            n = 0
            if t.has_old_version():
                n = r.test_starlight(t, history=discard if k == 0 else step_starlight, use_old=True, r=n, debug=False)
            r.test_starlight(t, history=discard if k == 0 else step_starlight_update, use_old=False, r=n, debug=False)

            r.service.reset_latency_bandwidth()  # REMOVE DELAY

            r.service.kill_starlight()

        # vanilla
        for k in range(t.rounds + 1):
            r.service.reset_container_service()

            r.service.set_latency_bandwidth(t.rtt[i])  # ADD DELAY

            n = 0
            if t.has_old_version():
                n = r.test_vanilla(t, history=discard if k == 0 else step_vanilla, use_old=True, r=n, debug=False)
            r.test_vanilla(t, history=discard if k == 0 else step_vanilla_update, use_old=False, r=n, debug=False)

            r.service.reset_latency_bandwidth()  # REMOVE DELAY

        # wget
        for k in range(t.rounds):
            r.service.set_latency_bandwidth(t.rtt[i])
            r.test_wget(t, history=discard if k == 0 else step_wget, use_old=True)
            r.test_wget(t, history=discard if k == 0 else step_wget_update, use_old=False)
            r.service.reset_latency_bandwidth()

        # save results
        pool_starlight.append(step_starlight)
        pool_vanilla.append(step_vanilla)
        pool_estargz.append(step_estargz)
        pool_wget.append(step_wget)

        pool_starlight_update.append(step_starlight_update)
        pool_vanilla_update.append(step_vanilla_update)
        pool_estargz_update.append(step_estargz_update)
        pool_wget_update.append(step_wget_update)

        df1, df2, df3, df4 = t.save_results(
            pool_estargz, pool_starlight, pool_vanilla, pool_wget,
            i + 1, "-deploy"
        )
        t.plot_results(df1, df2, df3, df4, "-deploy")

        df1, df2, df3, df4 = t.save_results(
            pool_estargz_update, pool_starlight_update, pool_vanilla_update, pool_wget_update,
            i + 1, "-update"
        )
        t.plot_results(df1, df2, df3, df4, "-update")

    r.service.reset_container_service()
    r.service.reset_latency_bandwidth()
