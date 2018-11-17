# A script to verify whether system output is strongly or eventually consistent

import re
from sys import stdin

# The system is strongly consistent if every read returns the last written
# value for the requested key.
stronglyConsistent = True

# The system is consistent with eventual consistency if every read returns a
# value which has been previously written for the requested key. Note that this
# alone does not prove the system is eventually consistent.
mightBeEventuallyConsistent = True

writtenValues = {}

pattern = re.compile(r"""(?P<timestamp>[0-9]+)\s(?P<type>[a-zA-Z]+)\s\{id:(?P<id>[0-9]+)\ssrc:(?P<src>[0-9]+)\sdest:(?P<dest>[0-9]+)\sdemuxKey:(?P<demuxKey>[0-9]+)\skey:\[(?P<key>[0-9]+(\s[0-9]+)*)\]\svalue:\[(?P<value>([0-9]+(\s[0-9]+)*)?)\]\sok:(?P<ok>[a-z]+)\}""")

for line in stdin:
    m = pattern.match(line)

    if m == None:
        print(line[:-1])
        continue

    msgType = int(m.group('demuxKey'))
    key = m.group('key').split(' ')
    value = m.group('value').split(' ')
    ok = m.group('ok')

    if msgType == 3 and ok == 'true':
        if key in writtenValues:
            values = writtenValues[key]
            if key not in values:
                mightBeEventuallyConsistent = False
            if key != values[-1]:
                stronglyConsistent = False
        else:
            print('ERROR: Non-error value returned for un-written key')
    elif msgType == 4 and ok == 'true':
        if key in writtenValues:
            writtenValues[key].append(value)
        else:
            writtenValues[key] = [value]

if stronglyConsistent:
    print('Strong consistency test passed')
elif mightBeEventuallyConsistent:
    print('Strong consistency test failed, but eventual consistency test passed')
else:
    print('All consistency tests failed!')
