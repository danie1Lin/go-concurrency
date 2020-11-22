package main

import (
	"fmt"
	"os"
	"os/signal"
	"runtime"
	"runtime/pprof"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
)

const sched = true

func main() {
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	if f, err := os.Create("./memprof"); err != nil {
		panic(err)
	} else {
		defer f.Close()
		runtime.GC()
		if err := pprof.WriteHeapProfile(f); err != nil {
			panic(err)
		}
	}

	if f, err := os.Create("./cpuprof1"); err != nil {
		panic(err)
	} else {
		defer f.Close()
		if err := pprof.StartCPUProfile(f); err != nil {
			panic(err)
		}
		defer pprof.StopCPUProfile()
	}
	lock := SimpleMutex{}

	waitCh := make(chan struct{})
	sum := 0
	t := 100000 // 這個數字不schedule就會卡死 cpu 100%
	s := time.Now()
	go func() {
		wg := sync.WaitGroup{}
		wg.Add(t)
		for i := 0; i < t; i++ {
			go func(i int) {
				lock.Lock()
				sum += 1
				lock.Unlock()
				wg.Done()
			}(i)
		}
		wg.Wait()
		close(waitCh)
	}()
	select {
	case <-waitCh:
		fmt.Println("done")
	case <-sigs:
		fmt.Println("interrupt")
	}
	fmt.Println("lock version sum:", sum, "time", time.Since(s), "OutOfSync", lock.outOfSync)
	return

}

type SimpleMutex struct {
	lock      int32
	outOfSync int32
}

func (m *SimpleMutex) Unlock() {
	for {
		old := m.lock
		if atomic.CompareAndSwapInt32(&m.lock, old, 0) {
			// if not locked
			if old != 1 {
				panic("already unlock")
			}
			return
		} else {
			// value need resync
			atomic.AddInt32(&m.outOfSync, 1)
		}
	}
}

func (m *SimpleMutex) Lock() {
	for {
		old := m.lock
		if atomic.CompareAndSwapInt32(&m.lock, old, 1) {
			if old == 0 {
				//fmt.Println("get")
				return
			} else {
				if sched {
					runtime.Gosched()
					// Not desire old state
				}
			}
		} else {
			atomic.AddInt32(&m.outOfSync, 1)
			// value need resync
		}
	}
}
