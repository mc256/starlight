import time
from typing import Callable
from datetime import date
import subprocess

import matplotlib.pyplot as plt
import numpy as np
import pandas as pd


class ContainerExperiment:
    # STARGZ_SUFFIX = "-stargz"
    STARGZ_SUFFIX = "-starlight"
    STARLIGHT_SUFFIX = "-starlight"

    def __init__(self, image_name, ready_keyword, version, old_version):
        self.ready_keyword = ready_keyword
        self.image_name = image_name
        self.version = version
        self.old_version = old_version
        self.has_mounting = False
        # self.rtt = [2, 25, 50, 75, 100, 125, 150, 175, 200, 225, 250, 275, 300]
        self.rtt = [2, 50, 100, 150, 200, 250, 300]
        self.rounds = 20
        self.expected_max_start_time = 30
        self.mounting = []
        self.has_args = False
        self.args = []

        self.experiment_name = ""

        self.workload_enabled = False
        self.workload_fn: Callable[[int, str, bool], None] or None = None
        self.workload_p = None

        self.update_experiment_name()

        self.action_history = []

    def set_experiment_name(self, name):
        self.experiment_name = name

    def update_experiment_name(self):
        today = date.today().strftime("%m%d")
        if self.old_version == "":
            self.experiment_name = "%s-%s--deploy-%s-r%d" % (self.image_name, today, self.version, self.rounds)
        else:
            self.experiment_name = "%s-%s--%s_%s-r%d" % (
                self.image_name, today, self.version, self.old_version, self.rounds)

        if self.workload_enabled is True and self.workload_fn is not None:
            self.experiment_name = self.experiment_name + '-wl'

    def set_mounting_points(self, mp=None):
        if mp is None:
            return
        self.mounting = mp
        self.has_mounting = True

    def set_args(self, args=None):
        if args is None:
            return
        self.has_args = True
        self.args = args

    def get_starlight_image(self, old=False):
        if old:
            if self.old_version == "":
                raise AssertionError("It should have an old image")
            return "%s:%s%s" % (self.image_name, self.old_version, self.STARLIGHT_SUFFIX)
        else:
            return "%s:%s%s" % (self.image_name, self.version, self.STARLIGHT_SUFFIX)

    def get_stargz_image(self, old=False):
        if old:
            if self.old_version == "":
                raise AssertionError("It should have an old image")
            return "%s:%s%s" % (self.image_name, self.old_version, self.STARGZ_SUFFIX)
        else:
            return "%s:%s%s" % (self.image_name, self.version, self.STARGZ_SUFFIX)

    def get_vanilla_image(self, old=False):
        if old:
            if self.old_version == "":
                raise AssertionError("It should have an old image")
            return "%s:%s" % (self.image_name, self.old_version)
        else:
            return "%s:%s" % (self.image_name, self.version)

    def has_old_version(self):
        return self.old_version != ""

    def has_workload(self):
        return self.workload_enabled

    def enable_workload(self):
        self.workload_enabled = True
        self.update_experiment_name()

    def load_results(self, suffix="", data_path="./pkl"):
        df1 = pd.read_pickle("%s/%s%s-%d.pkl" % (data_path, self.experiment_name, suffix, 1))
        df2 = pd.read_pickle("%s/%s%s-%d.pkl" % (data_path, self.experiment_name, suffix, 2))
        df3 = pd.read_pickle("%s/%s%s-%d.pkl" % (data_path, self.experiment_name, suffix, 3))
        df4 = pd.read_pickle("%s/%s%s-%d.pkl" % (data_path, self.experiment_name, suffix, 4))
        return df1, df2, df3, df4  # vanilla, estargz, starlight, wget

    def add_event(self, method="", event="", rtt=0, seq=0, ts=time.time(), delta=0.0):
        self.action_history.append([method, event, rtt, seq, ts, delta])

    def save_event(self, suffix=""):
        ddf = pd.DataFrame(self.action_history, columns=['method', 'event', 'rtt', 'round', 'ts', 'delta'])
        ddf.to_csv("./csv/%s%s-bundle.csv" % (self.experiment_name, suffix))

    def save_results(self, performance_estargz, performance_starlight, performance_vanilla, performance_wget,
                     position=1, suffix=""):
        estargz_np = np.array(performance_estargz)
        starlight_np = np.array(performance_starlight)
        vanilla_np = np.array(performance_vanilla)
        wget_np = np.array(performance_wget)

        df1 = pd.DataFrame(vanilla_np.T, columns=self.rtt[:position])
        df2 = pd.DataFrame(estargz_np.T, columns=self.rtt[:position])
        df3 = pd.DataFrame(starlight_np.T, columns=self.rtt[:position])
        df4 = pd.DataFrame(wget_np.T, columns=self.rtt[:position])

        df1.to_pickle("./pkl/%s%s-%d.pkl" % (self.experiment_name, suffix, 1))
        df2.to_pickle("./pkl/%s%s-%d.pkl" % (self.experiment_name, suffix, 2))
        df3.to_pickle("./pkl/%s%s-%d.pkl" % (self.experiment_name, suffix, 3))
        df4.to_pickle("./pkl/%s%s-%d.pkl" % (self.experiment_name, suffix, 4))

        return df1, df2, df3, df4

    def plot_single_result(self, step_starlight, step_vanilla, step_estargz, step_wget, suffix=""):
        df_avg = pd.DataFrame({
            "estargz": step_estargz,
            "starlight": step_starlight,
            "vanilla": step_vanilla,
            "wget": step_wget,
        })

        fig, (ax1) = plt.subplots(ncols=1, figsize=(4, 4), dpi=300)

        max_delay = self.expected_max_start_time

        fig.suptitle("%s%s" % (self.experiment_name, suffix))
        ax1.set_ylim([0, max_delay])
        ax1.set_ylabel('startup time (s)')
        ax1.set_xlabel('method')

        df_avg.plot(kind='box', ax=ax1, grid=True)
        ax1.title.set_text("fixed latency")

        fig.tight_layout()
        fig.savefig("./plot/%s%s.png" % (self.experiment_name, suffix), facecolor='w', transparent=False)

        plt.close(fig)

    def plot_results(self, df1, df2, df3, df4, suffix=""):
        df_avg = pd.DataFrame({
            'vanilla': df1.mean(),
            'estargz': df2.mean(),
            'starlight': df3.mean(),
            'wget': df4.mean(),
        },
            index=self.rtt
        )

        df1_q = df1.quantile([0.1, 0.9])
        df2_q = df2.quantile([0.1, 0.9])
        df3_q = df3.quantile([0.1, 0.9])
        df4_q = df4.quantile([0.1, 0.9])

        max_delay = self.expected_max_start_time

        fig, (ax1) = plt.subplots(ncols=1, figsize=(4, 4), dpi=300)

        fig.suptitle("%s%s" % (self.experiment_name, suffix))
        ax1.set_xlim([0, 300])
        ax1.set_ylim([0, max_delay])
        ax1.set_ylabel('startup time (s)')
        ax1.set_xlabel('RTT (ms)')

        ax1.fill_between(df1_q.columns, df1_q.loc[0.1], df1_q.loc[0.9], alpha=0.25)
        ax1.fill_between(df2_q.columns, df2_q.loc[0.1], df2_q.loc[0.9], alpha=0.25)
        ax1.fill_between(df3_q.columns, df3_q.loc[0.1], df3_q.loc[0.9], alpha=0.25)
        ax1.fill_between(df4_q.columns, df4_q.loc[0.1], df4_q.loc[0.9], alpha=0.25)

        df_avg.plot(kind='line', ax=ax1, grid=True, marker=".")
        ax1.legend(loc='upper left')
        ax1.title.set_text("mean & quantile[0.1,0.9]")

        fig.tight_layout()
        fig.savefig("./plot/%s%s.png" % (self.experiment_name, suffix), facecolor='w', transparent=False)

        plt.close(fig)

    def __repr__(self):
        return "ContainerExperiment<%s>" % self.experiment_name

    ####################################################################################################
    # FS Benchmark
    ####################################################################################################
    def workload_wait(self, debug):
        a, b = self.workload_p.communicate()
        if debug:
            print("-------------------------------- Workload begin")
            if a is not None:
                print(a.decode("utf-8"), end="")
            if b is not None:
                print(b.decode("utf-8"), end="")
            print("-------------------------------- Workload end", end="")


class ContainerExperimentX(ContainerExperiment):
    def __init__(self, img_name, img_type, download, tag, tag_old, mounts, ready, args=None, expectation=60,
                 workload_fn: Callable[[int, str, bool], subprocess.Popen] = None):
        """
        Create and experiment configuration
        Parameters
        ----------
        img_name : str
            Number of rows/columns of the subplot grid.

        img_type: str
            Category of the Image, based on HelloBench [FAST'16] or your paper. Not used

        download : str
            How many downloads from. Not used

        tag : str
            Version of the new container image. It should have no optimized suffix. Out program will add that
            (i.e. -starlight or -estargz) for you.

        tag_old : str
            Same as tag but an older version.

        mounts: List[MountingPoint]
            mounting points, the experiment will prepare external mounting point and remove those folder once finished.

        ready: str
            the line of text where container tells us it is ready.

        args: List[str] or None
            the parameters that can be executed.

        expectation: int
            expect when it will finish for the upper limit of the y-axis in plotting

        workload_fn: Callable[[int, str, bool], subprocess.Popen] or None
            start a workload with the program. [sequence number, method, debug].

        """
        super().__init__(
            img_name,
            ready,
            tag,
            tag_old
        )
        self.set_mounting_points(mounts)
        self.expected_max_start_time = expectation
        self.set_args(args)
        self.workload_fn = workload_fn
