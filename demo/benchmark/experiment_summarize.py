import matplotlib.pyplot as plt
import pandas as pd
import numpy as np

if __name__ == '__main__':
    services = [
        "rabbitmq-0508--3.8.14_3.8.13-r20",
        "alpine-0509--3.13.5_3.13.4-r20",
        "cassandra-0508--3.11.10_3.11.9-r20",
        "mariadb-0507--10.5.9_10.5.8-r20",
        "postgres-0507--13.2_13.1-r20",
        "rabbitmq-0508--3.8.14_3.8.13-r20",
        "redis-0508--6.2.2_6.2.1-r20",
        "registry-0509--2.7.1_2.7.0-r20",
        "ubuntu-0509--focal-20210416_focal-20210401-r20"
    ]

    base_dir = "/home/maverick/Desktop/us-east-1.starlight.yuri.moe/pkl"

    for s in services:
        suffix = "-update"
        df1 = pd.read_pickle("%s/%s%s-%d.pkl" % (base_dir, s, suffix, 1))
        df2 = pd.read_pickle("%s/%s%s-%d.pkl" % (base_dir, s, suffix, 2))
        df3 = pd.read_pickle("%s/%s%s-%d.pkl" % (base_dir, s, suffix, 3))
        df4 = pd.read_pickle("%s/%s%s-%d.pkl" % (base_dir, s, suffix, 4))
        # vanilla, estargz, starlight, wget

        df_avg = pd.DataFrame({
            "estargz": df2.mean() / df1.mean(),
            "starlight": df3.mean() / df1.mean(),
            "vanilla": df1.mean() / df1.mean(),
            "wget": df4.mean() / df1.mean(),
        })
        print(df_avg)
        break
