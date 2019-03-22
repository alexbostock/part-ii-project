import matplotlib.pyplot as plt
import re
import sys

prt = re.compile('.*read\sresponse\stime\s\(ms\):\s(?P<l>[0-9]+)')
pwt = re.compile('.*write\sresponse\stime\s\(ms\):\s(?P<l>[0-9]+)')

prr = re.compile('.*\((?P<r>[0-9]+\.[0-9])%\)\sreads')
pwr = re.compile('.*\((?P<r>[0-9]+\.[0-9])%\)\swrites')

rate = int(sys.argv[1])

strict_vws = []
strict_read_latencies = []
strict_write_latencies = []
strict_read_success_rates = []
strict_write_success_rates = []

vrs = []
vws = []

read_latencies = []
write_latencies = []

read_success_rates = []
write_success_rates = []

for vw in range(51, 100, 5):
    strict_vws.append(vw)
    vr = 100-vw+1

    filename = f'p-normal/{rate}/strict-{vr}-{vw}.txt'
    data = open(filename, 'r')

    for line in data:
        m = prt.match(line)
        if m != None:
            strict_read_latencies.append(int(m.group('l')))

        m = pwt.match(line)
        if m != None:
            strict_write_latencies.append(int(m.group('l')))

        m = prr.match(line)
        if m != None:
            strict_read_success_rates.append(float(m.group('r')))

        m = pwr.match(line)
        if m != None:
            strict_write_success_rates.append(float(m.group('r')))

fig, ax1 = plt.subplots()

ax1.scatter(strict_vws, strict_read_latencies, label=f'Latency of read transactions', c='r')
ax1.scatter(strict_vws, strict_write_latencies, label=f'Latency of write transactions', c='g')

ax1.set_xlabel('Write quorum size')
ax1.set_ylabel('Mean latency (ms)')

ax2 = ax1.twinx()

ax2.scatter(strict_vws, strict_read_success_rates, label='Read transaction success rate', c='b')
ax2.scatter(strict_vws, strict_write_success_rates, label='Write transaction success rate', c='y')

ax2.set_ylabel('Transaction success rate (%)')

ax1.set_title(f'{rate} transactions per second')
fig.legend()#loc='upper left')
fig.tight_layout()
plt.show()
