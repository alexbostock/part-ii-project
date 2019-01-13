# A script to parse output from the main system, read from stdin

import re
from sys import stdin

readTransactionTimes = {}
writeTransactionTimes = {}

pattern = re.compile(r"""(?P<timestamp>[0-9]+)\s(?P<type>[a-zA-Z]+)\s\{Id:(?P<id>[0-9]+)\sSrc:(?P<src>[0-9]+)\sDest:(?P<dest>[0-9]+)\sDemuxKey:(?P<demuxKey>[a-zA-Z]+)\sKey:\[(?P<key>[0-9]+(\s[0-9]+)*)\]\sValue:\[(?P<value>([0-9]+(\s[0-9]+)*)?)\]\sOk:(?P<ok>[a-z]+)\}""")

numReadRequests = 0
numWriteRequests = 0
numReadResponses = 0
numWriteResponses = 0
numReadErrors = 0
numWriteErrors = 0

totalTimeTaken = 0

for line in stdin:
    m = pattern.match(line)

    if m == None:
        print(line[:-1])
        continue

    timestamp = int(m.group('timestamp'))
    id = m.group('id')
    msgType = m.group('demuxKey')
    ok = m.group('ok')

    if msgType == 'clientReadRequest':
        numReadRequests += 1

        if id in readTransactionTimes:
            print('Unexpected duplicate request ID', id)
        else:
            readTransactionTimes[id] = timestamp
    elif msgType == 'clientWriteRequest':
        numWriteRequests += 1

        if id in writeTransactionTimes:
            print('Unexpected duplicate request ID', id)
        else:
            writeTransactionTimes[id] = timestamp
    elif msgType == 'clientReadResponse':
        numReadResponses += 1

        if id in readTransactionTimes:
            readTransactionTimes[id] = timestamp - readTransactionTimes[id]
        else:
            print('Unexpected response before corresponding request', id)

        if ok == 'false':
            numReadErrors += 1
    elif msgType == 'clientWriteResponse':
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

if numReadResponses > 0:
    print(numReadErrors, 'responses to reads were errors', '(' + str(100*numReadErrors/numReadResponses) + '%)')

if numWriteResponses > 0:
    print(numWriteErrors, 'responses to writes were errors', '(' + str(100*numWriteErrors/numWriteResponses) + '%)')

readTransactionTimes = readTransactionTimes.values()
writeTransactionTimes = writeTransactionTimes.values()

if len(readTransactionTimes) > 0:
    meanReadResponseTime = 0
    for time in readTransactionTimes:
        meanReadResponseTime += time

    meanReadResponseTime /= len(readTransactionTimes)

    print('Mean read response time (microseconds):', meanReadResponseTime)

if len(writeTransactionTimes) > 0:
    meanWriteResponseTime = 0
    for time in writeTransactionTimes:
        meanWriteResponseTime += time

    meanWriteResponseTime /= len(writeTransactionTimes)

    print('Mean write response time (microseconds):', meanWriteResponseTime)
