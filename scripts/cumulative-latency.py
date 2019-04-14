import math
import matplotlib.pyplot as plt
from matplotlib.ticker import PercentFormatter
import re
import sys

transaction_pattern = re.compile('(?P<start>[0-9]+) (?P<end>[0-9]+) (?P<type>[a-z]+).+ (?P<res>[a-z]+)$')

fileprefix = sys.argv[1]

succ_read_latencies = []
fail_read_latencies = []
succ_write_latencies = []
fail_write_latencies = []

for i in range(10):
    filepath = f'{fileprefix}-{i}.txt'
    data = open(filepath, 'r')

    for line in data:
        m = transaction_pattern.match(line)
        if m == None:
            continue

        latency = int(m.group('end')) - int(m.group('start'))
        ok = m.group('res') in ['true', 'success']

        if m.group('type') == 'read':
            if ok:
                succ_read_latencies.append(latency / 1000)
            else:
                fail_read_latencies.append(latency / 1000)
        else:
            if ok:
                succ_write_latencies.append(latency / 1000)
            else:
                fail_write_latencies.append(latency / 1000)

plt.hist(succ_read_latencies, len(succ_read_latencies), cumulative=True, histtype='step', density=True, label='Successful reads')
plt.hist(succ_write_latencies, len(succ_write_latencies), cumulative=True, histtype='step', density=True, label='Successful writes')

#plt.hist(fail_read_latencies, 100, cumulative=True, histtype='step', density=True, label='Failed reads')
#plt.hist(fail_write_latencies, 100, cumulative=True, histtype='step', density=True, label='Failed writes')

#plt.xscale('log')

print(len(succ_read_latencies) / (len(succ_read_latencies) + len(fail_read_latencies)))
print(len(succ_write_latencies) / (len(succ_write_latencies) + len(fail_write_latencies)))

plt.axhline(0.99, color='r', linestyle='dashed', label='99th percentile')

plt.xlabel('Latency (ms)')
plt.ylabel('Cumulative Probability')

plt.gca().yaxis.set_major_formatter(PercentFormatter(1))

plt.title(fileprefix)

plt.legend(loc='lower center')
plt.show()
