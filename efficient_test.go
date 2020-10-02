package main

import (
	"fmt"
	"math/rand"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
)

// golang intXXX, map, bool or pointer to anything assignments are atomic as of go1.15. This is not guaranteed by language spec & may change in future.
// This is a benchmark test, how efficient it will be to keep a CopyOnWrite Map compared to standard sync.Map or atomic.Value.
// Our usecase is some config kind of map which hardly changes. Too frequent `Get` and too infrequent `Put` for keys. getToPutRatio >>> 1000.
// Keys may or may not be disjoint.

// ************************** R E S U L T **************************
// Our efficient map is about 1%-2% efficient than AtomicMap which is 2%-5% efficient than sync.SyncMap

//Mapper is common map interface of all benchmarked implementations.
type Mapper interface {
	Get(key string) (interface{}, bool)
	Put(key string, val interface{})
}

// ************************** I M P L M E N T A T I O N S **************************

//SyncMap is implemented using standard sync.Map. No CopyOnWrite
type SyncMap struct {
	m sync.Map
}

func NewSyncMap() *SyncMap { return &SyncMap{sync.Map{}} }
func (sm *SyncMap) Get(key string) (interface{}, bool) {
	return sm.m.Load(key)
}
func (sm *SyncMap) Put(key string, val interface{}) {
	sm.m.Store(key, val)
}

//AtomicMap is CopyOnWrite map implemented using atomic.Value
type AtomicMap struct {
	av    *atomic.Value
	mutex sync.Mutex
}

func NewAtomicMap() *AtomicMap {
	av := &atomic.Value{}
	av.Store(map[string]interface{}{})
	return &AtomicMap{av: av}
}
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

// EfficientMap is implemented exploiting the fact that map assignments are atomic (against language spec. -race flag will complain)
// CopyOnWrite.
type EfficientMap struct {
	m     map[string]interface{} // Keep plain map
	mutex sync.Mutex
}

func NewEfficientMap() *EfficientMap { return &EfficientMap{m: map[string]interface{}{}} }
func (em *EfficientMap) Get(key string) (interface{}, bool) {
	ret, ok := em.m[key] // -race would complain !
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
	em.m = copy // -race would complain !
}

// ************************** B E N C H M A R K **************************

const getToPutRatio int32 = 20000 // How many `Get` calls against `Put` call.
const size int = 100              // Size of Map.

var am = NewAtomicMap()
var em = NewEfficientMap()
var sm = NewSyncMap()

func init() {
	f := func(m Mapper) {
		for i := 0; i < size; i++ {
			m.Put(strconv.Itoa(i), strconv.Itoa(i))
		}
	}
	f(am)
	f(em)
	f(sm)
}

func BenchmarkAtomicMap(b *testing.B) {
	bench(b, am)
}
func BenchmarkEfficientMap(b *testing.B) {
	bench(b, em)
}
func BenchmarkSyncMap(b *testing.B) {
	bench(b, sm)
}
func bench(b *testing.B, m Mapper) {
	benchFixedThreads(b, m)
	// benchVariableThreads(b, m)
}
func benchFixedThreads(b *testing.B, m Mapper) {
	b.ReportAllocs()
	wg := sync.WaitGroup{}
	for i := 0; i < 24; i++ { //Fixed threads
		wg.Add(1)
		go func() {
			defer wg.Done()
			for n := 0; n < b.N; n++ {
				v := strconv.Itoa(rand.Intn(size * size * size))
				if rand.Int31n(getToPutRatio+1) == 0 {
					m.Put(v, v)
				} else {
					v2, ok := m.Get(v)
					if rand.Int31n(100) > 100 { //Fooling compiler
						fmt.Println("Never written", v2, ok)
					}
				}
			}
		}()
	}
	wg.Wait()
}

func benchVariableThreads(b *testing.B, m Mapper) {
	b.ReportAllocs()
	wg := sync.WaitGroup{}
	for i := 0; i < b.N; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for n := 0; n < 10000; n++ { //Fixed Iterations
				v := strconv.Itoa(rand.Intn(size * size * size))
				if rand.Int31n(getToPutRatio+1) == 0 {
					m.Put(v, v)
				} else {
					v2, ok := m.Get(v)
					if rand.Int31n(100) > 100 { //Fooling compiler
						fmt.Println("Never written", v2, ok)
					}
				}
			}
		}()
	}
	wg.Wait()
}
