from common import Runner
from test_cases import *

if __name__ == '__main__':
    t = TestMySQL("8.0.24", "8.0.23")
    t.experiment_name = "mysql-0429--update-8.0.24_8.0.23-r20"
    #t = TestRedis("6.0", "5.0")
    #t.experiment_name = "redis-0429--update-6.0_5.0-r20"
    r = Runner()
    df1, df2, df3, df4 = t.load_results()
    t.plot_results(df1, df2, df3, df4)
