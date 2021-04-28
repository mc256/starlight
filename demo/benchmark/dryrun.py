from common import Runner
from testcases import *

if __name__ == '__main__':
    t = TestRedis("6.0", "5.0")

    t.set_experiment_name("dryrun")
    r = Runner()
    history_temp = []

    r.service.reset_latency_bandwidth()
    print("hello")

    # estargz
    r.service.reset_container_service()
    r.service.start_grpc_estargz()

    n = r.sync_pull_estargz(t, 0, False)

    r.service.set_latency_bandwidth(10)
    r.test_estargz(t, history_temp, n, False)
    r.service.reset_latency_bandwidth()

    r.service.kill_estargz()

    # starlight
    r.service.reset_container_service()
    r.service.start_grpc_starlight()

    n = r.sync_pull_starlight(t, 0, False)

    r.service.set_latency_bandwidth(10)
    r.test_starlight(t, history_temp, n, False)
    r.service.reset_latency_bandwidth()

    r.service.kill_starlight()

    # vanilla
    r.service.reset_container_service()

    n = r.sync_pull_vanilla(t, 0, False)

    r.service.set_latency_bandwidth(10)
    r.test_vanilla(t, history_temp, n, False)
    r.service.reset_latency_bandwidth()

    # wget
    r.service.set_latency_bandwidth(10)
    r.test_wget(t, history_temp)
    r.service.reset_latency_bandwidth()

    print(history_temp)
