import numpy as np
import matplotlib.pyplot as plt
import re
import sys

pr = re.compile('.*\((?P<r>[0-9]+\.[0-9])%\)\sreads')
pw = re.compile('.*\((?P<r>[0-9]+\.[0-9])%\)\swrites')

dir = sys.argv[1]
tx_type = sys.argv[2]
rate = int(sys.argv[3])

vrs = []
vws = []

read_success_rates = []
write_success_rates = []

for vw in range(51, 100, 5):
    vws.append(vw)

    reads = []
    writes = []

    for vr in [2, 5, 10, 20, 30, 40, 50]:
        if vw == 51:
            vrs.append(vr)

        if dir == 'convergence':
            filename = f'p-{dir}/{rate}/convergence-{vr}-{vw}.txt'
        else:
            filename = f'p-{dir}/{rate}/sloppy-{vr}-{vw}.txt'
        data = open(filename, 'r')

        for line in data:
            m = pr.match(line)
            if m != None:
                throughput = rate * float(m.group('r')) / 100
                reads.append(throughput)

            m = pw.match(line)
            if m != None:
                throughput = rate * float(m.group('r')) / 100
                writes.append(throughput)

    read_success_rates.append(reads)
    write_success_rates.append(writes)

fig, ax = plt.subplots()

if tx_type == 'read':
    throughputs = np.array(read_success_rates)
else:
    throughputs = np.array(write_success_rates)

im = ax.imshow(throughputs)

cbar = ax.figure.colorbar(im, ax=ax)

ax.set_xticks(np.arange(len(vrs)))
ax.set_yticks(np.arange(len(vws)))
ax.set_xticklabels(vrs)
ax.set_yticklabels(vws)

plt.setp(ax.get_xticklabels(), rotation=45, ha="right", rotation_mode="anchor")

#for i in range(len(vws)):
#    for j in range(len(vrs)):
#        text = ax.text(j, i, round(throughputs[i, j], 1), ha="center", va="center", color="w")

if tx_type == 'read':
    ax.set_title(f'Read throughput at rate {rate}')
else:
    ax.set_title(f'Write throughput at rate {rate}')

ax.set_xlabel('Read quorum size')
ax.set_ylabel('Write quorum size')

fig.tight_layout()
plt.show()
