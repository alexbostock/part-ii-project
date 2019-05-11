import math
import matplotlib.pyplot as plt
import os
import re
import sys

transaction_pattern = re.compile('(?P<start>[0-9]+) (?P<end>[0-9]+) (?P<type>[a-z]+).+ (?P<res>[a-z]+)$')

def find_percentiles(data, highs, meds, lows):
    data.sort()

    if len(data) > 0:
        lows.append(data[math.ceil(len(data) * 1/100) - 1] / 1000)
        meds.append(data[math.ceil(len(data) * 50/100) - 1] / 1000)
        highs.append(data[math.ceil(len(data) * 99/100) - 1] / 1000)

        highs[-1] = highs[-1] - meds[-1]
        lows[-1] = meds[-1] - lows[-1]
    else:
        lows.append(None)
        meds.append(None)
        highs.append(None)

def mask(x, y1, y2, y3):
    indices = []
    for i in range(len(y1)):
        if y1[i] == None:
            indices.append(i)

    indices.reverse()

    x = x.copy()

    for i in indices:
        del x[i]
        del y1[i]
        del y2[i]
        del y3[i]

    return x

fig, axes = plt.subplots(nrows=2, ncols=4, sharex=True, sharey='row')

row_num = 0
col_num = 0

#for n in range(10, 101, 10):
for n in range(10, 101, 30):
    for w in [n, int(n / 2 + 1)]:
        ax = axes[row_num][col_num]
        row_num += 1
        if row_num == 2:
            row_num = 0
            col_num += 1

        rates = []

        succ_read_meds = []
        succ_read_lows = []
        succ_read_highs = []
        fail_read_meds = []
        fail_read_lows = []
        fail_read_highs = []

        succ_write_meds = []
        succ_write_lows = []
        succ_write_highs = []
        fail_write_meds = []
        fail_write_lows = []
        fail_write_highs = []

        for rate in range(25, 301, 25):
            rates.append(rate)

            succ_read_latencies = []
            fail_read_latencies = []
            succ_write_latencies = []
            fail_write_latencies = []

            for i in range(10):
                filepath = f'high-n/{rate}/{n}-1-{w}-{i}.txt'
                data = open(filepath, 'r')

                for line in data:
                    m = transaction_pattern.match(line)
                    if m == None:
                        continue

                    latency = int(m.group('end')) - int(m.group('start'))
                    ok = m.group('res') in ['true', 'success']

                    if m.group('type') == 'read':
                        if ok:
                            succ_read_latencies.append(latency)
                        else:
                            fail_read_latencies.append(latency)
                    else:
                        if ok:
                            succ_write_latencies.append(latency)
                        else:
                            fail_write_latencies.append(latency)

            find_percentiles(succ_read_latencies, succ_read_highs, succ_read_meds, succ_read_lows)
            find_percentiles(fail_read_latencies, fail_read_highs, fail_read_meds, fail_read_lows)
            find_percentiles(succ_write_latencies, succ_write_highs, succ_write_meds, succ_write_lows)
            find_percentiles(fail_write_latencies, fail_write_highs, fail_write_meds, fail_write_lows)

        ax.set_title(f'N = {n}, R = 1, W = {w}')
        ax.set_yscale('log')

        if sys.argv[1] == 'succ':
            r = mask(rates, succ_read_highs, succ_read_meds, succ_read_lows)
            ax.scatter(r, succ_read_meds, label='Successful read transactions')
            ax.errorbar(r, succ_read_meds, yerr=(succ_read_lows, succ_read_highs), linestyle='None', capsize=5)

            r = mask(rates, succ_write_highs, succ_write_meds, succ_write_lows)
            ax.scatter(r, succ_write_meds, label='Successful write transactions')
            ax.errorbar(r, succ_write_meds, yerr=(succ_write_lows, succ_write_highs), linestyle='None', capsize=5)
        else:
            r = mask(rates, fail_read_highs, fail_read_meds, fail_read_lows)
            ax.scatter(r, fail_read_meds, label='Failed reads')
            ax.errorbar(r, fail_read_meds, yerr=(fail_read_lows, fail_read_highs), linestyle='None', capsize=5)

            r = mask(rates, fail_write_highs, fail_write_meds, fail_write_lows)
            ax.scatter(r, fail_write_meds, label='Failed writes')
            ax.errorbar(r, fail_write_meds, yerr=(fail_write_lows, fail_write_highs), linestyle='None', capsize=5)

for ax in axes[1]:
    ax.set_xlabel('Rate of transactions / second')

axes[0][0].set_ylabel('Latency (ms)')
axes[1][0].set_ylabel('Latency (ms)')

axes[0][0].legend(loc='upper left')
plt.show()
