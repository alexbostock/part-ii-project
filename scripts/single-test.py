import matplotlib.pyplot as plt
import pandas as pd
import re
import sys

pr = re.compile('(?P<start>[0-9]+) (?P<end>[0-9]+) read')
pw = re.compile('(?P<start>[0-9]+) (?P<end>[0-9]+) write')

filename = sys.argv[1]

read_times = []
read_latencies = []
write_times = []
write_latencies = []

f = open(filename, 'r')

for line in f:
    m = pr.match(line)
    if m != None:
        read_times.append(int(m.group('start')))
        read_latencies.append(int(m.group('end')) - read_times[-1])

    m = pw.match(line)
    if m != None:
        write_times.append(int(m.group('start')))
        write_latencies.append(int(m.group('end')) - read_times[-1])

def rolling_average(index, window, xs, ys):
    sum = 0
    num = 0

    i = index
    while i < len(ys) and xs[i] < xs[index] + window:
        sum += ys[i]
        num += 1
        i += 1

    i = index - 1
    while i >= 0 and xs[i] > xs[index] - window:
        sum += ys[i]
        num += 1
        i -= 1

    return sum / num

plt.xlabel('Transaction start time (s)')
plt.ylabel('Rolling average transaction latency (us)')

xs, ys = (list(t) for t in zip(*sorted(zip(read_times, read_latencies))))
rolling_means = []

for i in range(len(xs)):
    rolling_means.append(rolling_average(i, 1000000, xs, ys))

xs = list(map(lambda x: x / 1000000, xs))

plt.scatter(xs, rolling_means, label='Reads')

xs, ys = (list(t) for t in zip(*sorted(zip(write_times, write_latencies))))
rolling_means = []

for i in range(len(xs)):
    rolling_means.append(rolling_average(i, 10000000, xs, ys))

xs = list(map(lambda x: x / 1000000, xs))

plt.scatter(xs, rolling_means, label='Writes')

plt.legend()

plt.show()
