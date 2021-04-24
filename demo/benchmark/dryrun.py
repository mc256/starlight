import subprocess, os, signal
import time
import random
import numpy as np
import re
import pandas as pd
import matplotlib.pyplot as plt
import math
import common as c
import experiment as t

ROUNDS = 20
#EXPERIMENT_NAME = "Redis6.0-Update5.0-r%d-v10" % ROUNDS
EXPERIMENT_NAME = "mysql8.0.21-Update8.0.20-r%d-v10" % ROUNDS

performance_estargz = []
performance_starlight = []
performance_vanilla = []
performance_wget = []

temp = []
# latencies = [0, 25, 50, 75, 100, 125, 150, 175, 200, 225, 250, 275, 300, 325, 350]
latencies = [0, 50, 100, 150, 200, 250, 300, 350]

"""
class ExperimentConfig:
    STARGZ_GRPC = 'stargz-grpc'
    STARLIGHT_GRPC = 'starlight-grpc'
    NETWORK_DEVICE = "mpqemubr0"
    REGISTRY_SERVER = "container-worker.momoko"
    PROXY_SERVER = "container-worker.momoko"
    KEYWORD = "* Ready to accept connections"

    OLD_IMAGE_NAME = "redis:5.0"
    IMAGE_NAME = "redis:6.0"
"""
class ExperimentConfig:
    STARGZ_GRPC = 'stargz-grpc'
    STARLIGHT_GRPC = 'starlight-grpc'
    NETWORK_DEVICE = "mpqemubr0"
    REGISTRY_SERVER = "container-worker.momoko"
    PROXY_SERVER = "container-worker.momoko"
    KEYWORD = "socket: '/var/run/mysqld/mysqld.sock'  port: 3306  MySQL Community Server - GPL"

    OLD_IMAGE_NAME = "mysql:8.0.20"
    IMAGE_NAME = "mysql:8.0.21"

cfg = ExperimentConfig()
c.reset_container_service()
c.reset_latency_bandwidth(cfg)

# DRY RUN
grpc1 = c.start_grpc_estargz(cfg)
r = t.sync_pull_estargz(cfg, grpc1, cfg.OLD_IMAGE_NAME, 0, True)
c.set_latency_bandwidth(cfg, 200)
t.test_estargz(cfg, cfg.IMAGE_NAME, temp, r, True)
c.reset_latency_bandwidth(cfg)
c.kill_process(grpc1)
c.reset_container_service()

grpc2 = c.start_grpc_starlight(cfg)
r = t.sync_pull_starlight(cfg, grpc2, cfg.OLD_IMAGE_NAME, 0, True)
c.set_latency_bandwidth(cfg, 200)
t.test_starlight(cfg, cfg.IMAGE_NAME, temp, r, cfg.OLD_IMAGE_NAME, True, 0)
c.reset_latency_bandwidth(cfg)
c.kill_process(grpc2)
c.reset_container_service()

r = t.sync_pull_vanilla(cfg, cfg.OLD_IMAGE_NAME, 0, True)
c.set_latency_bandwidth(cfg, 200)
t.test_vanilla(cfg, cfg.IMAGE_NAME, temp, r, True)
c.reset_latency_bandwidth(cfg)
c.reset_container_service()

# Benchmark
for latency in latencies:
    print("latency: %dms" % latency)

    buffer_estargz = []
    buffer_starlight = []
    buffer_vanilla = []

    for i in range(ROUNDS):
        grpc1 = c.start_grpc_estargz(cfg)
        r = t.sync_pull_estargz(cfg, grpc1, cfg.OLD_IMAGE_NAME, 0, False)
        c.set_latency_bandwidth(cfg, latency)
        t.test_estargz(cfg, cfg.IMAGE_NAME, buffer_estargz, r, False)
        c.reset_latency_bandwidth(cfg)
        c.kill_process(grpc1)
        c.reset_container_service()

        grpc2 = c.start_grpc_starlight(cfg)
        r = t.sync_pull_starlight(cfg, grpc2, cfg.OLD_IMAGE_NAME, 0, False)
        c.set_latency_bandwidth(cfg, latency)
        t.test_starlight(cfg, cfg.IMAGE_NAME, buffer_starlight, r, cfg.OLD_IMAGE_NAME, False, 0)
        c.reset_latency_bandwidth(cfg)
        c.kill_process(grpc2)
        c.reset_container_service()

        r = t.sync_pull_vanilla(cfg, cfg.OLD_IMAGE_NAME, 0, False)
        c.set_latency_bandwidth(cfg, latency)
        t.test_vanilla(cfg, cfg.IMAGE_NAME, buffer_vanilla, r, False)
        c.reset_latency_bandwidth(cfg)
        c.reset_container_service()

    performance_estargz.append(buffer_estargz)
    performance_starlight.append(buffer_starlight)
    performance_vanilla.append(buffer_vanilla)

estargz_np = np.array(performance_estargz)
starlight_np = np.array(performance_starlight)
vanilla_np = np.array(performance_vanilla)

df1 = pd.DataFrame(vanilla_np.T, columns=latencies)
df2 = pd.DataFrame(estargz_np.T, columns=latencies)
df3 = pd.DataFrame(starlight_np.T, columns=latencies)

df_avg = pd.DataFrame({
    'vanilla': df1.mean(),
    'estargz': df2.mean(),
    'starlight': df3.mean(),
},
    index=latencies
)

df1.to_pickle("./pkl/%s-%d.pkl" % (EXPERIMENT_NAME, 1))
df2.to_pickle("./pkl/%s-%d.pkl" % (EXPERIMENT_NAME, 2))
df3.to_pickle("./pkl/%s-%d.pkl" % (EXPERIMENT_NAME, 3))

df1_q = df1.quantile([0.1, 0.9])
df2_q = df2.quantile([0.1, 0.9])
df3_q = df3.quantile([0.1, 0.9])

max_delay = 12

fig, (ax1) = plt.subplots(ncols=1, sharey=True, figsize=(4, 4), dpi=80)

fig.suptitle("%s" % EXPERIMENT_NAME)
ax1.set_xlim([0, 350])
ax1.set_ylim([0, max_delay])
ax1.set_ylabel('startup time (s)')

ax1.fill_between(df1_q.columns, df1_q.loc[0.1], df1_q.loc[0.9], alpha=0.25)
ax1.fill_between(df2_q.columns, df2_q.loc[0.1], df2_q.loc[0.9], alpha=0.25)
ax1.fill_between(df3_q.columns, df3_q.loc[0.1], df3_q.loc[0.9], alpha=0.25)

df_avg.plot(kind='line', ax=ax1, grid=True)
ax1.legend(loc='upper left')
ax1.title.set_text("mean & quantile[0.1,0.9]")

fig.tight_layout()
fig.savefig("./plot/%s.png" % EXPERIMENT_NAME, facecolor='w', transparent=False)
