from common import Runner
from common import MountingPoint as MP
from common import ContainerExperimentX as X

if __name__ == '__main__':
    t = X(
        'mysql', 'database',
        '1B',
        '8.0.24', '8.0.23',
        [
            MP("/var/lib/mysql", False, "rw", "999:999"),
            MP("/run/mysqld", False, "rw", "999:999")
        ],
        "port: 3306  MySQL Community Server - GPL",
        None,
        30
    )
    # t.experiment_name = "mysql-0429--update-8.0.24_8.0.23-r20"
    r = Runner()
    df1, df2, df3, df4 = t.load_results()
    t.plot_results(df1, df2, df3, df4)
