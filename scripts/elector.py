import math
import re

pattern = re.compile('(?P<start>[0-9]+) (?P<end>[0-9]+) (?P<type>[a-z]+).+ (?P<res>[a-z]+)$')

def stats(title, dir):
    read_latencies = []
    write_latencies = []

    succ_reads = 0
    succ_writes = 0
    total_reads = 0
    total_writes = 0

    for i in range(10):
        filepath = f'{dir}/t-{i}.txt'
        data = open(filepath, 'r')

        for line in data:
            m = pattern.match(line)
            if m == None:
                continue

            latency = int(m.group('end')) - int(m.group('start'))
            latency /= 1000

            if m.group('type') == 'read':
                total_reads += 1

                if m.group('res') in ['true', 'success']:
                    succ_reads += 1
                    read_latencies.append(latency)
            else:
                total_writes += 1

                if m.group('res') in ['true', 'success']:
                    succ_writes += 1
                    write_latencies.append(latency)

    read_latencies.sort()
    write_latencies.sort()

    print(title)
    print()
    print('Read yield:', round(100 * succ_reads / total_reads, 1))
    print('Write yield:', round(100 * succ_writes / total_writes, 1))
    print('Median latency of successful reads (ms):', round(read_latencies[math.ceil(len(read_latencies) * 50/100)]))
    print('Median latency of successful writes (ms):', round(write_latencies[math.ceil(len(write_latencies) * 50/100)]))
    print('99th percentile latency of successful reads (ms):', round(read_latencies[math.ceil(len(read_latencies) * 99/100)]))
    print('99th percentile latency of successful writes (ms):', round(write_latencies[math.ceil(len(write_latencies) * 99/100)]))
    print()

stats('No Leader', 'elector/none')
stats('Bully', 'elector/bully')
stats('Ring', 'elector/ring')
