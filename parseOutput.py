#!/usr/bin/env python3
# A script to parse transaction logs from stdin
# eg. go run main.go | python3 parseOutput.py

import pandas as pd
import matplotlib.pyplot as plt

import json

import re
import sys

READ = 0
WRITE = 1

def request_type(s):
    if s == 'read':
        return READ
    elif s == 'write':
        return WRITE
    else:
        print('Invalid request type:', s)
        sys.exit(1)

SUCCESS = 0
FAILURE = 1
UNKNOWN = 2

def res_type(s):
    if s in ['true', 'success']:
        return SUCCESS
    elif s in ['false', 'error']:
        return FAILURE
    elif s == 'unknown':
        return UNKNOWN
    else:
        print('Invalid response type:', s)
        sys.exit(2)

def encode_byte_array(s):
    arr = s[1:-1].split(' ')

    res = 0
    for byte in arr:
        if byte != '':
            res *= 256
            res += int(byte)

    return res

int_pattern = re.compile('(?P<val>[0-9]+)(?P<tail>.*)')
str_pattern = re.compile('(?P<val>[a-zA-Z]+)(?P<tail>.*)')
byte_array_pattern = re.compile('(?P<val>\[([0-9]+(\s[0-9]+)*)?\])(?P<tail>.*)')

json_pattern = re.compile('^\{.*\}$')

two_ints_pattern = re.compile('^[0-9]+\s[0-9]+')

class Transaction:
    def parse(self, pattern):
        if self.line[0] == ' ':
            self.line = self.line[1:]

        m = pattern.match(self.line)
        if m == None:
            print('Error parsing', self.line)
            print(pattern)
            sys.exit(3)

        self.line = m.group('tail')
        return m.group('val')

    def __init__(self, line):
        self.line = line

        self.start_time = int(self.parse(int_pattern))
        self.end_time = int(self.parse(int_pattern))
        self.type = request_type(self.parse(str_pattern))
        self.key = encode_byte_array(self.parse(byte_array_pattern))
        self.value = encode_byte_array(self.parse(byte_array_pattern))
        self.timestamp = self.parse(int_pattern)
        self.ok = res_type(self.parse(str_pattern))

        self.time_taken = self.end_time - self.start_time

    def print(self):
        print(self.start_time, self.end_time, self.type, self.key, self.value, self.timestamp)

total_reads = 0
total_writes = 0

successful_reads = 0
successful_writes = 0
failed_writes = 0

total_time_taken = 0
time_taken_reads = 0
time_taken_writes = 0

transactions = {} # Keyed by key

jsons = []

for line in sys.stdin:
    m = str_pattern.match(line)
    if m != None:
        continue

    m = json_pattern.match(line)

    if m == None:
        m = two_ints_pattern.match(line)
        if m == None:
            continue

        t = Transaction(line)

        if t.key not in transactions:
            transactions[t.key] = set()
        transactions[t.key].add(t)

        total_time_taken = max(total_time_taken, t.end_time)

        if t.type == READ:
            total_reads += 1
            time_taken_reads += t.time_taken

            if t.ok == SUCCESS:
                successful_reads += 1
        else:
            total_writes += 1
            time_taken_writes += t.time_taken

            if t.ok == SUCCESS:
                successful_writes += 1
            elif t.ok == FAILURE:
                failed_writes += 1

    else:
        jsons.append(line)

def div(x, y):
    if x == y == 0:
        return 0
    else:
        return x / y

print('Total time taken (milliseconds):', int(total_time_taken/1000))
print(total_reads, 'reads and', total_writes, 'writes processed')
print('Average read response time (ms):', int(div(time_taken_reads/1000, total_reads)))
print('Average write response time (ms):', int(div(time_taken_writes/1000, total_writes)))
print(successful_reads, '(' + str(int(100*div(successful_reads, total_reads))) + '%) reads were successful')
print(successful_writes, '(' + str(int(100*div(successful_writes, total_writes))) + '%) writes were successful')
print(total_writes-successful_writes-failed_writes, 'writes have unknown response')

print()

strongly_consistent = True
eventually_consistent = True

for key, ts in transactions.items():
    # For each pair of successful transactions
    for t in ts:
        if t.ok in [FAILURE, UNKNOWN]:
            continue
        for u in ts:
            if u.ok in [FAILURE, UNKNOWN]:
                continue

            # Test consistency

            # Same timestamp => same value
            if t.timestamp == u.timestamp and t.value != u.value:
                print('Write conflict detected.')
                strongly_consistent = False
                eventually_consistent = False

            if t.start_time > u.start_time:
                continue

            # t happens before u and strong_consistency => t.timestamp >= u.timestamp
            if t.end_time < u.start_time and t.timestamp > u.timestamp:
                strongly_consistent = False

if strongly_consistent:
    print('Output appears to be strongly consistent.')
elif eventually_consistent:
    print('Output appears to be eventually consistent.')
else:
    print('Output appears to be neither strongly consistent nor eventually consistent.')

if len(jsons) > 1 and len(sys.argv) > 1:
    messages = json.loads(jsons[0])
    nodes = json.loads(jsons[1])

    pd.DataFrame(nodes).transpose().plot(kind='area')
    plt.show()
