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

fig, ((ax0, ax1, ax2, ax3), (ax4, ax5, ax6, ax7)) = plt.subplots(nrows=2, ncols=4, sharex=True, sharey='row')

params = [
    (3, 1, 3, ax0),
    (3, 1, 2, ax1),
    (5, 1, 3, ax2),
    (5, 1, 5, ax3),
    (3, 2, 2, ax4),
    (5, 2, 4, ax5),
    (5, 3, 3, ax6),
    (5, 2, 3, ax7),
] # n, r, w, axes

for p in params:
    n, r, w, ax = p

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

    if os.path.isfile(f'{sys.argv[1]}/350/{n}-{r}-{w}-0.txt'):
        rate_range = range(25, 301, 25)
    else:
        rate_range = range(25, 251, 25)

    for rate in rate_range:
        rates.append(rate)

        succ_read_latencies = []
        fail_read_latencies = []
        succ_write_latencies = []
        fail_write_latencies = []

        for i in range(10):
            filepath = f'{sys.argv[1]}/{rate}/{n}-{r}-{w}-{i}.txt'
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

    ax.set_title(f'N = {n}, R = {r}, W = {w}')

    if sys.argv[2] == 'succ':
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

    ax.set_yscale('log')

    if sys.argv[1] == 'normal':
        ax.set_ylim(10, 11000)

for ax in [ax4, ax5, ax6, ax7]:
    ax.set_xlabel('Rate of transactions / second')
for ax in [ax0, ax4]:
    if sys.argv[2] == 'succ':
        ax.set_ylabel('Latency (ms)')
    else:
        ax.set_ylabel('Latency (ms) (log scale)')

ax2.legend(loc='upper left')
plt.show()
