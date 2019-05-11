import matplotlib.pyplot as plt
import os
import re
import sys

num_transactions_p = re.compile('(?P<reads>[0-9]+) reads and (?P<writes>[0-9]+) writes processed')
succ_reads_p = re.compile('(?P<num>[0-9]+) \(.+%\) reads.+')
succ_writes_p = re.compile('(?P<num>[0-9]+) \(.+%\) writes.+')

def mmm(data):
    maximum = data[0]
    minimum = data[0]
    total = 0

    for item in data:
        maximum = max(maximum, item)
        minimum = min(minimum, item)
        total += item

    return total / len(data), maximum, minimum

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
    mean_r_yields = []
    max_r_yields = []
    min_r_yields = []
    mean_w_yields = []
    max_w_yields = []
    min_w_yields = []

    if os.path.isfile(f'p-{sys.argv[1]}/350/{n}-{r}-{w}-0.txt'):
        rate_range = range(25, 351, 25)
    else:
        rate_range = range(25, 251, 25)

    for rate in rate_range:
        rates.append(rate)

        read_yields = []
        write_yields = []

        for i in range(10):
            filepath = f'p-{sys.argv[1]}/{rate}/{n}-{r}-{w}-{i}.txt'
            data = open(filepath, 'r')

            for line in data:
                m = num_transactions_p.match(line)
                if m != None:
                    total_reads = int(m.group('reads'))
                    total_writes = int(m.group('writes'))

                m = succ_reads_p.match(line)
                if m != None:
                    succ_reads = int(m.group('num'))

                m = succ_writes_p.match(line)
                if m != None:
                    succ_writes = int(m.group('num'))

            read_yields.append(100 * succ_reads / total_reads)
            write_yields.append(100 * succ_writes / total_writes)

        mean, maximum, minimum = mmm(read_yields)
        mean_r_yields.append(mean)
        max_r_yields.append(maximum - mean)
        min_r_yields.append(mean - minimum)

        mean, maximum, minimum = mmm(write_yields)
        mean_w_yields.append(mean)
        max_w_yields.append(maximum - mean)
        min_w_yields.append(mean - minimum)

    ax.set_title(f'N = {n}, R = {r}, W = {w}')
    ax.scatter(rates, mean_r_yields, label='Read transactions')
    ax.scatter(rates, mean_w_yields, label='Write transactions')

    ax.errorbar(rates, mean_r_yields, yerr=(min_r_yields, max_r_yields), linestyle='None', capsize=5)
    ax.errorbar(rates, mean_w_yields, yerr=(min_w_yields, max_w_yields), linestyle='None', capsize=5)

for ax in [ax4, ax5, ax6, ax7]:
    ax.set_xlabel('Rate of transactions / second')
for ax in [ax0, ax4]:
    ax.set_ylabel('Percentage yield')

ax1.legend(loc='lower right')
plt.show()
