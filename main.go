package main

import (
	"bytes"
	"fmt"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"
)

type Sink struct {
	metricsQueue chan string
}

func NewSink() *Sink {
	s := &Sink{
		metricsQueue: make(chan string, 2048),
	}
	go s.flushMetrics()
	return s
}

func (s *Sink) Shutdown() {
	close(s.metricsQueue)
}

func (s *Sink) pushMetric(m string) {
	select {
	case s.metricsQueue <- m:
	default:
	}
}

const flushInterval = 10 * time.Second

func (s *Sink) flushMetrics() {
	buf := bytes.NewBuffer(nil)
	ticker := time.NewTicker(flushInterval)
	defer ticker.Stop()

	for {
		select {
		case metric, ok := <-s.metricsQueue:
			if !ok {
				fmt.Print("not ok")
				break
			}

			buf.WriteString(metric)

		case <-ticker.C:
			if buf.Len() == 0 {
				continue
			}

			_, err := buf.WriteTo(os.Stdout)
			buf.Reset()
			if err != nil {
				fmt.Printf("buffer flush failure: %v", err)
				break
			}
		}
	}
}

type Metrics struct {
	sink *Sink
}

func (mm *Metrics) pushMetric(m string) {
	mm.sink.pushMetric(m)
}

var globalMetrics *Metrics

func init() {
	globalMetrics = &Metrics{sink: NewSink()}
}

func main() {
	sigs := make(chan os.Signal, 1)
	done := make(chan struct{}, 1)

	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		termSig := <-sigs
		fmt.Println(termSig)
		done <- struct{}{}
	}()

	go collectStats()

	<-done
	fmt.Println("Exiting...")
}

func collectStats() {
	for {
		time.Sleep(2 * time.Second)
		collectRuntimeMetrics()
	}
}

func collectRuntimeMetrics() {
	numGoroutines := runtime.NumGoroutine()
	globalMetrics.pushMetric(fmt.Sprintf("num goroutines=%d\n", numGoroutines))

	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)
	globalMetrics.pushMetric(fmt.Sprintf("alloc_bytes=%d\n", memStats.Alloc))
	globalMetrics.pushMetric(fmt.Sprintf("sys_bytes=%d\n", memStats.Sys))
	// globalMetrics.pushMetric(fmt.Sprintf("malloc_count=%d\n", memStats.Mallocs))
	// globalMetrics.pushMetric(fmt.Sprintf("free_count=%d\n", memStats.Frees))
	// globalMetrics.pushMetric(fmt.Sprintf("heap_objects=%d\n", memStats.HeapObjects))
}
