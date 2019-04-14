import matplotlib.pyplot as plt
import pandas as pd
import re
import sys

pr = re.compile('(?P<start>[0-9]+) (?P<end>[0-9]+) read \[.*\] \[.*\] [0-9]+ (?P<res>[a-z]+)')
pw = re.compile('(?P<start>[0-9]+) (?P<end>[0-9]+) write \[.*\] \[.*\] [0-9]+ (?P<res>[a-z]+)')

node_fail_p = re.compile('Node [0-9]+ failed.+')
node_recover_p = re.compile('Node [0-9]+ recovered')

filename = sys.argv[1]

read_times = []
read_latencies = []
write_times = []
write_latencies = []

node_fail_times = []
node_recover_times = []
partition_times = []
partition_recover_times = []

f = open(filename, 'r')

for line in f:
    m = pr.match(line)
    if m != None:
        read_times.append(int(m.group('start')))
        if m.group('res') == 'true':
            read_latencies.append(100)
        else:
            read_latencies.append(0)

    m = pw.match(line)
    if m != None:
        write_times.append(int(m.group('start')))
        if m.group('res') == 'success':
            write_latencies.append(100)
        else:
            write_latencies.append(0)

    m = node_fail_p.match(line)
    if m != None:
        node_fail_times.append(read_times[-1])

    m = node_recover_p.match(line)
    if m != None:
        node_recover_times.append(read_times[-1])

    if line == 'Partition created\n':
        partition_times.append(read_times[-1])

    if line == 'Partition recovered\n':
        partition_recover_times.append(read_times[-1])

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
plt.ylabel('Rolling average percentage yield')

xs, ys = (list(t) for t in zip(*sorted(zip(read_times, read_latencies))))
rolling_means = []

for i in range(len(xs)):
    rolling_means.append(rolling_average(i, 5000000, xs, ys))

xs = list(map(lambda x: x / 1000000, xs))

plt.scatter(xs, rolling_means, label='Reads')

xs, ys = (list(t) for t in zip(*sorted(zip(write_times, write_latencies))))
rolling_means = []

for i in range(len(xs)):
    rolling_means.append(rolling_average(i, 10000000, xs, ys))

xs = list(map(lambda x: x / 1000000, xs))

plt.scatter(xs, rolling_means, label='Writes')

for time in node_fail_times:
    plt.axvline(x=time / 1000000, color='m')

for time in node_recover_times:
    plt.axvline(x=time / 1000000, color='c')

for time in partition_times:
    plt.axvline(x=time / 1000000, color='r')

for time in partition_recover_times:
    plt.axvline(x=time / 1000000, color='g')

plt.legend()

plt.show()
