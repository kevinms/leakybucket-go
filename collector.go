package leakybucket

import (
	"container/heap"
	"fmt"
	"sync"
	"time"
)

//TODO: Finer grained locking.

type bucketMap map[string]*LeakyBucket

// A Collector can keep track of multiple LeakyBucket's. The caller does not
// directly interact with the buckets, but instead addresses them by a string
// key (e.g. IP address, hostname, hash, etc.) that is passed to most Collector
// methods.
//
// All Collector methods are goroutine safe.
type Collector struct {
	buckets  bucketMap
	heap     priorityQueue
	rate     float64
	capacity int64
	lock     sync.Mutex
	quit     chan bool
}

// Creates a new Collector. When new buckets are created within the Collector,
// they will be assigned the capacity and rate of the Collector. A Collector
// does not provide a way to change the rate or capacity of bucket's within it.
// If different rates or capacities are required, either use multiple
// Collector's or manage your own LeakyBucket's.
//
// If deleteEmptyBuckets is true, a concurrent goroutine will be run that
// watches for bucket's that become empty and automatically removes them,
// freeing up memory resources.
func NewCollector(rate float64, capacity int64, deleteEmptyBuckets bool) *Collector {
	c := &Collector{
		buckets:  make(bucketMap),
		heap:     make(priorityQueue, 0, 4096),
		rate:     rate,
		capacity: capacity,
		quit:     make(chan bool),
	}

	if deleteEmptyBuckets {
		c.PeriodicPrune(time.Second)
	}

	return c
}

// Frees the collector's resources. If the collector was created with
// deleteEmptyBuckets = true, then the goroutine looking for empty buckets,
// will be stopped.
func (c *Collector) Free() {
	c.Reset()
	close(c.quit)
}

// Removes all internal buckets and resets the collector back to as if it was
// just created.
func (c *Collector) Reset() {
	c.lock.Lock()
	defer c.lock.Unlock()

	// Let the garbage collector do all the work.
	c.buckets = make(bucketMap)
	c.heap = make(priorityQueue, 0, 4096)
}

// Returns the collector's capacity.
func (c *Collector) Capacity() int64 {
	return c.capacity
}

// Returns the collector's rate.
func (c *Collector) Rate() float64 {
	return c.rate
}

// Returns the remaining capacity of the internal bucket associated with key.
// If key is not associated with a bucket internally, it is treated as being
// empty.
func (c *Collector) Remaining(key string) int64 {
	return c.capacity - c.Count(key)
}

// Returns the count of the internal bucket associated with key. If key is not
// associated with a bucket internally, it is treated as being empty.
func (c *Collector) Count(key string) int64 {
	c.lock.Lock()
	defer c.lock.Unlock()

	b, _ := c.buckets[key]
	if b == nil {
		return 0
	}

	return b.Count()
}

// Returns how much time must pass until the internal bucket associated with
// key is empty. If key is not associated with a bucket internally, it is
// treated as being empty.
func (c *Collector) TillEmpty(key string) time.Duration {
	c.lock.Lock()
	defer c.lock.Unlock()

	b, _ := c.buckets[key]
	if b == nil {
		return 0
	}

	return b.TillEmpty()
}

// Removes the internal bucket associated with key. If key is not associated
// with a bucket internally, nothing is done.
func (c *Collector) Remove(key string) {
	c.lock.Lock()
	defer c.lock.Unlock()

	b, _ := c.buckets[key]
	if b == nil {
		return
	}

	delete(c.buckets, b.key)
	heap.Remove(&c.heap, b.index)
}

// Add 'amount' to the internal bucket associated with key, up to it's
// capacity. Returns how much was added to the bucket. If the return is less
// than 'amount', then the bucket's capacity was reached.
//
// If key is not associated with a bucket internally, a new bucket is created
// and amount is added to it.
func (c *Collector) Add(key string, amount int64) int64 {
	c.lock.Lock()
	defer c.lock.Unlock()

	b, _ := c.buckets[key]

	if b == nil {
		// Create a new bucket.
		b = &LeakyBucket{
			key:      key,
			capacity: c.capacity,
			rate:     c.rate,
			p:        now(),
		}
		c.heap.Push(b)
		c.buckets[key] = b
	}

	n := b.Add(amount)

	if n > 0 {
		heap.Fix(&c.heap, b.index)
	}

	return n
}

// Removes all empty buckets in the collector.
func (c *Collector) Prune() {
	c.lock.Lock()
	for c.heap.Peak() != nil {
		b := c.heap.Peak()

		if now().Before(b.p) {
			// The bucket isn't empty.
			break
		}

		fmt.Printf("Pruning '%s'\n", b.key)
		// The bucket should be empty.
		delete(c.buckets, b.key)
		heap.Remove(&c.heap, b.index)
	}
	c.lock.Unlock()
}

// Runs a concurrent goroutine that calls Prune() at the given time interval.
func (c *Collector) PeriodicPrune(interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		for {
			select {
			case <-ticker.C:
				c.Prune()
			case <-c.quit:
				ticker.Stop()
				return
			}
		}
	}()
}
