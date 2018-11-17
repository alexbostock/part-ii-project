# A script to parse output from the main system, read from stdin

import re
from sys import stdin

readTransactionTimes = {}
writeTransactionTimes = {}

pattern = re.compile(r"""(?P<timestamp>[0-9]+)\s(?P<type>[a-zA-Z]+)\s\{id:(?P<id>[0-9]+)\ssrc:(?P<src>[0-9]+)\sdest:(?P<dest>[0-9]+)\sdemuxKey:(?P<demuxKey>[0-9]+)\skey:\[(?P<key>[0-9]+(\s[0-9]+)*)\]\svalue:\[(?P<value>([0-9]+(\s[0-9]+)*)?)\]\sok:(?P<ok>[a-z]+)\}""")

numReadRequests = 0
numWriteRequests = 0
numReadResponses = 0
numWriteResponses = 0
numReadErrors = 0
numWriteErrors = 0

totalTimeTaken = 0

for line in stdin:
    m = pattern.match(line)

    timestamp = int(m.group('timestamp'))
    id = m.group('id')
    msgType = int(m.group('demuxKey'))
    ok = m.group('ok')

    if msgType == 1:
        numReadRequests += 1

        if id in readTransactionTimes:
            print('Unexpected duplicate request ID', id)
        else:
            readTransactionTimes[id] = timestamp
    elif msgType == 2:
        numWriteRequests += 1

        if id in writeTransactionTimes:
            print('Unexpected duplicate request ID', id)
        else:
            writeTransactionTimes[id] = timestamp
    elif msgType == 3:
        numReadResponses += 1

        if id in readTransactionTimes:
            readTransactionTimes[id] = timestamp - readTransactionTimes[id]
        else:
            print('Unexpected response before corresponding request', id)

        if ok == 'false':
            numReadErrors += 1
    elif msgType == 4:
        numWriteResponses += 1

        if id in writeTransactionTimes:
            writeTransactionTimes[id] = timestamp - writeTransactionTimes[id]
        else:
            print('Unexpected response before corresponding request', id)

        if ok == 'false':
            numWriteErrors += 1
    else:
        print('Unexpected output:')
        print(line)

    totalTimeTaken = int(timestamp)

print('Time taken (milliseconds):', totalTimeTaken / 1000)
print(numReadResponses, 'reads and', numWriteResponses, 'writes responded to')

if numReadRequests != numReadResponses:
    print('ERROR: Number of read responses does not match number of requests')

if numWriteRequests != numWriteResponses:
    print('ERROR: Number of write responses does not match number of requests')

print(numReadErrors, 'responses to reads were errors', '(' + str(100*numReadErrors/numReadResponses) + '%)')

print(numWriteErrors, 'responses to writes were errors', '(' + str(100*numWriteErrors/numWriteResponses) + '%)')

readTransactionTimes = readTransactionTimes.values()
writeTransactionTimes = writeTransactionTimes.values()

meanReadResponseTime = 0
for time in readTransactionTimes:
    meanReadResponseTime += time

meanWriteResponseTime = 0
for time in writeTransactionTimes:
    meanWriteResponseTime += time

meanReadResponseTime /= len(readTransactionTimes)
meanWriteResponseTime /= len(writeTransactionTimes)

print('Mean read response time (microseconds):', meanReadResponseTime)
print('Mean write response time (microseconds):', meanWriteResponseTime)
