from common import Runner
from mounting_point import MountingPoint as M
from container_experiment import ContainerExperimentX as X

if __name__ == '__main__':
    t = X(
        'mariadb', 'database', '1B', '10.5.9', '10.5.8',
        [M("/var/lib/mysql", False, "rw", "999:999"), M("/run/mysqld", False, "rw", "999:999")],
        "port: 3306  mariadb.org binary distribution",
        None, 30
    )

    t.experiment_name = "mariadb-0504--10.5.9_10.5.8-r20"
    location = "tokyo3"
    job="deploy"
    r = Runner()
    vanilla, estargz, starlight, wget = t.load_results("-%s" % job, "/home/maverick/Desktop/%s/pkl" % location)
    print(starlight.describe())
    t.plot_single_result(
        starlight[2].to_numpy(),
        vanilla[2].to_numpy(),
        estargz[2].to_numpy(),
        wget[2].to_numpy(),
        "-%s-%s" % (job, location)
    )
