import math
import re
import matplotlib.pyplot as plt
import matplotlib.ticker as mtick
import scipy.stats as stats

pattern = re.compile('[0-9]+ [0-9]+ (?P<type>[a-z]+).+ (?P<res>[a-z]+)$')

def mean(xs):
    sum = 0
    n = 0

    for x in xs:
        sum += x
        n += 1

    return sum / n

def yields(dir):
    read_yields = []
    write_yields = []

    for i in range(20):
        succ_reads = 0
        succ_writes = 0
        total_reads = 0
        total_writes = 0

        filepath = f'{dir}/t-{i}.txt'
        data = open(filepath, 'r')

        for line in data:
            m = pattern.match(line)
            if m == None:
                continue

            if m.group('type') == 'read':
                total_reads += 1

                if m.group('res') in ['true', 'success']:
                    succ_reads += 1
            else:
                total_writes += 1

                if m.group('res') in ['true', 'success']:
                    succ_writes += 1

        read_yields.append(succ_reads / total_reads)
        write_yields.append(succ_writes / total_writes)

    return read_yields, write_yields

none_read, none_write = yields('elector/none')
ring_read, ring_write = yields('elector/ring')

print(mean(none_read), mean(ring_read))
print(mean(none_write), mean(ring_write))

print(stats.ranksums(ring_read, none_read))
print(stats.ranksums(ring_write, none_write))

fig, (axr, axw) = plt.subplots(ncols=2)

axr.hist([none_read, ring_read], 10, label=['No leader', 'Ring Election'])
axw.hist([none_write, ring_write], 10, label=['No leader', 'Ring Election'])

axr.xaxis.set_major_formatter(mtick.PercentFormatter(1))
axw.xaxis.set_major_formatter(mtick.PercentFormatter(1))

axr.set_xlabel('Read Yield')
axw.set_xlabel('Write Yield')
axr.set_ylabel('Frequency')
axw.set_ylabel('Frequency')

plt.legend()

#ax = [None] * 4
#fig, ((ax[0], ax[1]), (ax[2], ax[3])) = plt.subplots(ncols=2, nrows=2)
#
#stats.probplot(none_read, plot=ax[0])
#ax[0].set_title('Reads, no leader')
#
#stats.probplot(ring_read, plot=ax[1])
#ax[1].set_title('Reads, ring election')
#
#stats.probplot(none_write, plot=ax[2])
#ax[2].set_title('Writes, no leader')
#
#stats.probplot(ring_write, plot=ax[3])
#ax[3].set_title('Writes, ring election')
#
#for i in range(4):
#    ax[i].set_xlim(-1.6, 1.6)
#    ax[i].set_ylim(-1.6, 1.6)

plt.show()
