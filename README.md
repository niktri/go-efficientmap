# go-efficientmap
Threadsafe golang Map - Efficient safe read-write map than `sync.Map` or `atomic.Value` CopyOnWrite Map
See Implementation: https://github.com/niktri/go-efficientmap/blob/main/efficient_test.go

# What is Efficient Map ?

**EfficientMap** is a threadsafe CopyOnWrite map, allowing safe reads & writes concurrently. 
Exploiting the fact that golang assignments are atomic. Cool down! Yes they are not officially, but read on...

# Usecase 

- Keeping a Config settings/Cache, which hardly changes but read on every request. Too frequent `Get` and too infrequent `Put` for keys. `getToPutRatio >>> 1000.`
- Keys may or may not be disjoint.

# Plain golang map Implementation

- Concurrent reads are safe.
- It is a runtime panic if multiple go-routines are reading & writing.
- It is still a panic even if single go-routine is writing protected by mutex, if other go-routines are reading unprotected.

# sync.Map 

- golang [sync.Map](https://golang.org/pkg/sync/#Map) is drop-in placement for this usecase. Get and Store operations are already Threadsafe.
- Already threadsafe. No mutex required in client code.
- sync.Map is however better suited for disjoint set of keys.
```
    type SyncMap struct {m sync.Map}
    func NewSyncMap() *SyncMap { return &SyncMap{sync.Map{}} }
    func (sm *SyncMap) Get(key string) (interface{}, bool) {return sm.m.Load(key)}
    func (sm *SyncMap) Put(key string, val interface{}) {sm.m.Store(key, val)}
```
# atomic.Value 

- golang [atomic.Value](https://golang.org/pkg/sync/atomic/#Value) provides an atomic way to load/store anything. We can implement CopyOnWrite as follows:
- **`Get operation`** Load our map from atomic.Value. Now we can read this without any lock.
- **`Put operation`** Lock mutex. Load our map. Copy it. Update copy with new value. Atomically store into atomic.Value. Unlock mutex.
```
    type AtomicMap struct {av    *atomic.Value;mutex sync.Mutex}
    func (am *AtomicMap) Get(key string) (interface{}, bool) {
        ret, ok := am.av.Load().(map[string]interface{})[key]
        return ret, ok
    }
    func (am *AtomicMap) Put(key string, val interface{}) {
        am.mutex.Lock()
        defer am.mutex.Unlock()
        m := am.av.Load().(map[string]interface{})
        copy := make(map[string]interface{}, len(m))
        for k, v := range m {
            copy[k] = v
        }
        copy[key] = val
        am.av.Store(copy)
    }
```
# Efficient Map - Our Solution

- Since golang map assignments are atomic, we can implement above CopyOnWrite Solution without using `atomic.Value`.
```
    type EfficientMap struct {m     map[string]interface{};mutex sync.Mutex} // Keep plain map
    func (em *EfficientMap) Get(key string) (interface{}, bool) {
        ret, ok := em.m[key] // map read exploit !
        return ret, ok
    }
    func (em *EfficientMap) Put(key string, val interface{}) {
        em.mutex.Lock()
        defer em.mutex.Unlock()
        copy := make(map[string]interface{}, len(em.m))
        for k, v := range em.m {
            copy[k] = v
        }
        copy[key] = val
        em.m = copy // map assignment exploit !
    }
```
# Are golang assignments atomic ?
- Go language spec does not guarantee any assignments to be atomic. But..
- [Here](https://research.swtch.com/gorace) Russ Cox happens to know a var assignment to be atomic.
- As of go1.15 it seems(to me) that int family, map & pointers assignments are atomic. This may not be true for int64 on 32 or 16 bit systems. 
  `-race` would ofcourse complain this. So far never found any concurrency panics or wrong values. Please let me know if you find one.
- Interface, string, slices, structs are of course not atomic.
- This may change in future and it is dangerous to rely on this atomicity.

# Result
```
BenchmarkAtomicMap-12       	  149994	      8505 ns/op	     207 B/op	      24 allocs/op
BenchmarkEfficientMap-12    	  148770	      8350 ns/op	     207 B/op	      24 allocs/op
BenchmarkSyncMap-12         	  137811	      8651 ns/op	     203 B/op	      24 allocs/op
```
Occassionally Our Efficient map is about **1%-2% efficient** than AtomicMap which is 2%-5% efficient than sync.SyncMap.
- Results are not very consistent. We seldom achieved >2% performance gain.
- Also we assumed `getToPutRatio >> 1000`. For lower ratios `sync.Map` is best, *especially if keys are disjoint*.
- If map size is big CopyOnWrite is waste.

# Bottomline

Our [Efficient Map](https://github.com/niktri/go-efficientmap/blob/main/efficient_test.go) is good for academic interest. In practice, always use plain map with mutex or `sync.Map`.