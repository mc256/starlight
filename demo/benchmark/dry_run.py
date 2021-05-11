from common import ContainerExperimentX as X
from common import Runner
from common import MountingPoint as M

if __name__ == '__main__':
    # t = TestMySQL("8.0.24", "8.0.23")
    # t = X('nginx', 'web-server', '1B', '1.20.0', '1.19.10', [], "ready for start up")
    # t = X('httpd', 'web-server', '1B', '2.4.46', '2.4.43', [], "Command line: 'httpd -D FOREGROUND'")
    # t = X('redis', 'database', '1B', '6.2.2', '6.2.1',[M("/data")],"* Ready to accept connections",None, 10)
    # t.set_experiment_name("dry-run" + t.experiment_name)
    """
    X('alpine', 'distro', '1B', '3.13.5', '3.13.4', [
        M("",overwrite="type=bind,"
                       "src=/home/ubuntu/Development/starlight/demo/config/hello-entrypoint.sh,"
                       "dst=/entrypoint.sh,"
                       "options=rbind:ro"
        )
    ], "hello", ["/entrypoint.sh"]),
    X('busybox', 'distro', '1B', '1.32.1', '1.32.0', [
        M("",overwrite="type=bind,"
                       "src=/home/ubuntu/Development/starlight/demo/config/hello-entrypoint.sh,"
                       "dst=/entrypoint.sh,"
                       "options=rbind:ro"
        )
    ], "hello", ["/entrypoint.sh"]),
    """
    t = X('redis', 'database', '1B', '6.2.2', '6.2.1', [M("/data")], "* Ready to accept connections", None, 10)

    r = Runner()
    history_temp = []

    print("This is a dry run.")

    rtt = 150
    debug = True

    r.service.reset_latency_bandwidth(True)
    r.service.set_latency_bandwidth(rtt)

    print("RTT:%d" % rtt)

    # -------------------- starlight --------------------
    r.service.reset_container_service()
    r.service.start_grpc_starlight()

    n = 0
    if t.has_old_version():
        # #n = r.sync_pull_starlight(t, 0, True)
        n = r.test_starlight(t, history=history_temp, use_old=True, r=n, debug=debug, ycsb=True)
        r.service.set_latency_bandwidth(rtt)
        pass

    # r.service.set_latency_bandwidth(rtt, True)
    r.test_starlight(t, history=history_temp, use_old=False, r=n, debug=debug, ycsb=True)
    # r.service.reset_latency_bandwidth(True)

    r.service.kill_starlight()

    # -------------------- vanilla --------------------
    r.service.reset_container_service()

    n = 0
    if t.has_old_version():
        # #n = r.sync_pull_starlight(t, 0, True)
        n = r.test_vanilla(t, history=history_temp, use_old=True, r=n, debug=debug, ycsb=True)
        pass

    # r.service.set_latency_bandwidth(rtt, True)
    r.test_vanilla(t, history=history_temp, use_old=False, r=n, debug=debug, ycsb=True)
    # r.service.reset_latency_bandwidth(True)

    # -------------------- estargz --------------------
    r.service.reset_container_service()
    r.service.start_grpc_estargz()

    n = 0
    if t.has_old_version():
        # #n = r.sync_pull_estargz(t, 0, True)
        n = r.test_estargz(t, history=history_temp, use_old=True, r=n, debug=debug, ycsb=True)
        r.service.set_latency_bandwidth(rtt)
        pass

    # r.service.set_latency_bandwidth(rtt, True)
    r.test_estargz(t, history=history_temp, use_old=False, r=n, debug=debug, ycsb=True)
    # r.service.reset_latency_bandwidth(True)

    r.service.kill_estargz()

    # ----------------------------------------------------

    # print out results
    print(history_temp)

    r.service.reset_latency_bandwidth(True)
