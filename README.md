# A leaky bucket implementation in Go.

[![Build Status](https://travis-ci.com/kevinms/leakybucket-go.svg?branch=master)](https://travis-ci.com/kevinms/leakybucket-go)
[![Go Report Card](https://goreportcard.com/badge/github.com/kevinms/leakybucket-go)](https://goreportcard.com/report/github.com/kevinms/leakybucket-go)
[![GoDoc](https://godoc.org/github.com/kevinms/leakybucket-go?status.svg)](https://godoc.org/github.com/kevinms/leakybucket-go)

There are at least two different definitions of the leaky bucket algorithm. This package implements the leaky bucket as a meter. For more details see:

https://en.wikipedia.org/wiki/Leaky_bucket#As_a_meter

This means it is the exact mirror of a token bucket.

```go
// New LeakyBucket that leaks at the rate of 0.5/sec and a total capacity of 10.
b := NewLeakyBucket(0.5, 10)

b.Add(5)
b.Add(5)
// Bucket is now full!

n := b.Add(1)
// n == 0
```

A Collector is a convenient way to keep track of multiple LeakyBucket's.
Buckets are associated with string keys for fast lookup. It can dynamically
add new buckets and automatically remove them as they become empty, freeing
up resources.

```go
// New Collector that leaks at 1 MiB/sec, a total capacity of 10 MiB and
// automatic removal of bucket's when they become empty.
const megabyte = 1<<20
c := NewCollector(megabyte, megabyte*10, true)

// Attempt to add 100 MiB to a bucket associated with an IP.
n := c.Add("192.168.0.42", megabyte*100)

// 100 MiB is over the capacity, so only 10 MiB is actually added.
// n equals 10 MiB.
```
