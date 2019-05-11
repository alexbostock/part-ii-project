import re
import sys

def calculate_latency(start, end):
    s = start.split(':')
    s = 3600*int(s[0]) + 60*int(s[1]) + float(s[2])

    e = end.split(':')
    e = 3600*int(e[0]) + 60*int(e[1]) + float(e[2])

    return e - s

filepath_pattern = re.compile('convergence/(?P<rate>[0-9]+)/(?P<n>[0-9]+)-(?P<r>[0-9]+)-(?P<w>[0-9]+)-[0-9]+\.txt')

write_commit_pattern = re.compile('(?P<time>[0-9]{2}:[0-9]{2}:[0-9]{2}\.[0-9]{6}) (?P<id>[0-9]+) write commit (?P<key>\[.+\]) (?P<ts>[0-9]+)')
background_write_pattern = re.compile('(?P<time>[0-9]{2}:[0-9]{2}:[0-9]{2}\.[0-9]{6}) (?P<id>[0-9]+) background write (?P<key>\[.+\]) (?P<ts>[0-9]+)')

filepath = sys.argv[1]

m = filepath_pattern.match(filepath)
n, r, w = int(m.group('n')), int(m.group('r')), int(m.group('w'))

num_background_writes_required = (n - r + 1) - w

background_writes_done = {} # key -> node set
timestamp = {} # key -> Lamport timestamp
start_time = {} # key -> UTC timestamp

for line in open(filepath, 'r'):
    m = write_commit_pattern.match(line)
    if m != None:
        key, ts = m.group('key'), m.group('ts')
        background_writes_done[key] = set()
        timestamp[key] = ts
        start_time[key] = m.group('time')

    m = background_write_pattern.match(line)
    if m != None:
        key, ts = m.group('key'), m.group('ts')
        if timestamp[key] == ts:
            background_writes_done[key].add(m.group('id'))

        if len(background_writes_done[key]) == num_background_writes_required:
            latency = calculate_latency(start_time[key], m.group('time'))
            print(latency)
