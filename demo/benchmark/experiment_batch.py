from common import Runner
from common import ContainerExperimentX as X
from common import MountingPoint as M

"""
X(
    'redis', 'database', '1B', '6.2.2', '6.2.1',
    [M("/data")],
    "* Ready to accept connections",
    None, 10
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
    for t in [

    ]:

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

            r.service.set_latency_bandwidth(t.rtt[i])  # ADD DELAY
            # estargz
            for k in range(t.rounds + 1):
                r.service.reset_container_service()
                r.service.start_grpc_estargz()

                n = 0
                if t.has_old_version():
                    n = r.test_estargz(t, history=discard if k == 0 else step_estargz, use_old=True, r=n, debug=False)

                r.service.set_latency_bandwidth(t.rtt[i])  # ADD DELAY
                r.test_estargz(t, history=discard if k == 0 else step_estargz_update, use_old=False, r=n, debug=False)

                r.service.kill_estargz()

            # starlight
            for k in range(t.rounds + 1):
                r.service.reset_container_service()
                r.service.start_grpc_starlight()

                n = 0
                if t.has_old_version():
                    n = r.test_starlight(t, history=discard if k == 0 else step_starlight, use_old=True, r=n,
                                         debug=False)

                r.service.set_latency_bandwidth(t.rtt[i])  # ADD DELAY
                r.test_starlight(t, history=discard if k == 0 else step_starlight_update, use_old=False, r=n,
                                 debug=False)

                r.service.kill_starlight()

            # vanilla
            for k in range(t.rounds + 1):
                r.service.reset_container_service()

                n = 0
                if t.has_old_version():
                    n = r.test_vanilla(t, history=discard if k == 0 else step_vanilla, use_old=True, r=n, debug=False)
                r.test_vanilla(t, history=discard if k == 0 else step_vanilla_update, use_old=False, r=n, debug=False)

            # wget
            for k in range(t.rounds):
                r.test_wget(t, history=discard if k == 0 else step_wget, use_old=True)
                r.service.set_latency_bandwidth(t.rtt[i])  # ADD DELAY
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
