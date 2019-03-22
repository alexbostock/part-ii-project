import matplotlib.pyplot as plt
import re
import sys

pr = re.compile('.*\((?P<r>[0-9]+\.[0-9])%\)\sreads')
pw = re.compile('.*\((?P<r>[0-9]+\.[0-9])%\)\swrites')

dir = sys.argv[1] # Should be normal or nofail
tx_type = sys.argv[2] # Should be read or write

for rate in range(50, 300, 50):
    strict_vrs = []
    strict_read_success_rates = []
    strict_write_success_rates = []

    for vw in range(51, 100, 5):
        vr = 100-vw+1
        strict_vrs.append(vr)

        filename = f'p-{dir}/{rate}/strict-{vr}-{vw}.txt'
        data = open(filename, 'r')

        for line in data:
            m = pr.match(line)
            if m != None:
                strict_read_success_rates.append(float(m.group('r')))

            m = pw.match(line)
            if m != None:
                strict_write_success_rates.append(float(m.group('r')))

    for i in range(len(strict_vrs)):
        strict_read_success_rates[i] = rate * strict_read_success_rates[i] / 100
        strict_write_success_rates[i] = rate * strict_write_success_rates[i] / 100

    if tx_type == 'read':
        plt.scatter(strict_vrs, strict_read_success_rates, label=f'{rate} transactions / s')
    else:
        plt.scatter(strict_vrs, strict_write_success_rates, label=f'{rate} transactions / s')

plt.xlabel('Read quorum size')
if tx_type == 'read':
    plt.ylabel('Successful read transactions / s')
else:
    plt.ylabel('Successful write transactions / s')

plt.legend()
plt.show()
