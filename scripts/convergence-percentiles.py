import math
import matplotlib.pyplot as plt
from matplotlib.ticker import PercentFormatter
import re
import sys

def percentiles(x):
    x.sort()
    n = len(x)

    ps = [10, 50, 90, 95, 99]
    percentiles = []

    for p in ps:
        index = int(n * p / 100)
        percentiles.append(str(round(x[index], 2)))

    return percentiles

def read_times(fileprefix):
    times = []

    for i in range(10):
        filepath = f'{fileprefix}-{i}.txt'
        data = open(filepath, 'r')

        for line in data:
            times.append(1000 * float(line))

    return times

print('\\begin{longtable}{@{} l l l l l l l l l @{}}')
print('\\toprule')
print('N & R & W & Transaction Rate & \multicolumn{5}{c}{Latency at Percentile (ms)} \\\\')
print('& & & 10th & 50th & 90th & 95th & 99th \\\\')

params = [
    (3, 1, 2),
    (5, 1, 3),
    (5, 2, 3),
]

for p in params:
    n, r, w = p

    print('\\hline')

    for rate in range(50, 351, 50):
        filepath = f'c-convergence/{rate}/{n}-{r}-{w}'
        ps = ' & '.join(percentiles(read_times(filepath)))

        print(f'{n} & {r} & {w} & {rate} & {ps} \\\\')

print('\\bottomrule')
print('\\end{longtable}')

print()

print('\\begin{longtable}{@{} l l l l l l l @{}}')
print('\\toprule')
print('N & Transaction Rate & \multicolumn{5}{c}{Latency at Percentile (ms)} \\\\')
print('& & 10th & 50th & 90th & 95th & 99th \\\\')

for n in range(10, 101, 10):
    w = int(n / 2 + 1)

    for rate in range(50, 351, 50):
        filepath = f'c-convergence/{rate}/{n}-1-{w}'
        ps = ' & '.join(percentiles(read_times(filepath)))

        if rate == 50:
            print('\\hline')
            print(f'{n} & {rate} & {ps} \\\\')
        else:
            print(f'& {rate} & {ps} \\\\')

print('\\bottomrule')
print('\\end{longtable}')
