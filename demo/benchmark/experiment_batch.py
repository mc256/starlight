from runner import Runner
from benchmark_pop_bench import PopBench

if __name__ == '__main__':

    event_suffix = "-v8"

    for key in ['wordpress-fpm', 'openjdk']:
        t = PopBench[key]
        r = Runner()
        discard = []

        r.service.reset_latency_bandwidth()
        # t.rtt = [2]
        t.rounds = 3
        exp_methods = {'starlight', 'estargz', 'vanilla', 'wget'}
        # exp_methods = {'starlight'}
        t.update_experiment_name()

        print("Hello! This is Starlight Stage. We are running experiment:\n\t- %s" % t)

        for i in range(len(t.rtt)):
            print("RTT:%d" % t.rtt[i])

            r.service.set_latency_bandwidth(t.rtt[i])  # ADD DELAY
            if 'estargz' in exp_methods:
                # estargz
                for seq in range(t.rounds + 1):
                    r.service.reset_container_service()
                    r.service.start_grpc_estargz()

                    n = 0
                    if t.has_old_version():
                        n = r.test_estargz(
                            t,
                            seq == 0,
                            rtt=t.rtt[i],
                            seq=seq,
                            use_old=True,
                            r=n,
                            debug=False
                        )
                        pass

                    r.test_estargz(
                        t,
                        seq == 0,
                        rtt=t.rtt[i],
                        seq=seq,
                        use_old=False,
                        r=n,
                        debug=False
                    )

                    r.service.kill_estargz()
                    t.save_event(event_suffix)
            pass

            if 'starlight' in exp_methods:
                # starlight
                for seq in range(t.rounds + 1):
                    r.service.reset_container_service()
                    r.service.start_grpc_starlight()

                    n = 0
                    if t.has_old_version():
                        n = r.test_starlight(
                            t,
                            seq == 0,
                            rtt=t.rtt[i],
                            seq=seq,
                            use_old=True,
                            r=n,
                            debug=False
                        )
                        pass

                    r.test_starlight(
                        t,
                        seq == 0,
                        rtt=t.rtt[i],
                        seq=seq,
                        use_old=False,
                        r=n,
                        debug=False
                    )

                    r.service.kill_starlight()
                    t.save_event(event_suffix)
            pass

            if 'vanilla' in exp_methods:
                # vanilla
                for seq in range(t.rounds + 1):
                    r.service.reset_container_service()

                    n = 0
                    if t.has_old_version():
                        n = r.test_vanilla(
                            t,
                            seq == 0,
                            rtt=t.rtt[i],
                            seq=seq,
                            use_old=True,
                            r=n,
                            debug=False
                        )
                        pass

                    r.test_vanilla(
                        t,
                        seq == 0,
                        rtt=t.rtt[i],
                        seq=seq,
                        use_old=False,
                        r=n,
                        debug=False
                    )
                    t.save_event(event_suffix)
            pass

            if 'wget' in exp_methods:
                # wget
                for seq in range(t.rounds + 1):
                    r.test_wget(t, seq == 0, rtt=t.rtt[i], seq=seq, use_old=True)
                    r.test_wget(t, seq == 0, rtt=t.rtt[i], seq=seq, use_old=False)
                    t.save_event(event_suffix)
            pass
            
            r.service.reset_latency_bandwidth()

        r.service.reset_container_service()
        r.service.reset_latency_bandwidth()
