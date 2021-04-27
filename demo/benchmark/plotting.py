import numpy as np
import pandas as pd
import matplotlib.pyplot as plt


def save_result(performance_estargz, performance_starlight, performance_vanilla):
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
