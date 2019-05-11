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

fig, axes = plt.subplots(nrows=2, ncols=4, sharex=True, sharey='row')

row_num = 0
col_num = 0

r = 1

#for n in range(10, 101, 10):
for n in range(10, 101, 30):
    for w in [n, int(n / 2 + 1)]:
        ax = axes[row_num][col_num]
        row_num += 1
        if row_num == 2:
            row_num = 0
            col_num += 1

        rates = []
        mean_r_yields = []
        max_r_yields = []
        min_r_yields = []
        mean_w_yields = []
        max_w_yields = []
        min_w_yields = []

        for rate in range(25, 301, 25):
            rates.append(rate)

            read_yields = []
            write_yields = []

            for i in range(10):
                filepath = f'p-high-n/{rate}/{n}-{r}-{w}-{i}.txt'
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

for ax in axes[1]:
    ax.set_xlabel('Rate of transactions / seconds')

axes[0][0].set_ylabel('Percentage yield')
axes[1][0].set_ylabel('Percentage yield')

axes[1][0].legend(loc='lower left')
plt.show()
