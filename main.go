package main

import (
	"fmt"
	"math/rand"
	"sync/atomic"
	"time"
)

type Stub struct{}

var STUB = Stub{}

func main() {
	// создадим общий канал в который будем закидывать задачи
	pool := NewWorkerPool(3)
	pool.Start()
	for range 20 {
		pool.Execute(func() any {
			x := rand.Int63n(5)
			time.Sleep(time.Duration(x) * time.Second)
			i := rand.Int()
			input := fmt.Sprintf("Some random value, %d", i)
			fmt.Println(input)
			return i
		})
	}
	time.Sleep(time.Minute)
	for data := range pool.outChan {
		fmt.Println("some value came, it is: ", data)
	}
	pool.Stop()
	
}

type WorkerPool struct {
	controlChan  chan command
	finishedChan chan Stub
	workerCount  *atomic.Int32
	outChan      chan any
}

func (w *WorkerPool) Execute(t task) {
	chanNum := w.workerCount.Add(1)
	go func(i int32) {
		<-w.finishedChan
		fmt.Printf("task number %d started\n", i)
		data := t()
		fmt.Printf("task number %d finished\n", i)
		w.outChan <- data
		w.finishedChan <- STUB
	}(chanNum)
}

func (w *WorkerPool) Inc() {
	w.controlChan <- Add
}
func (w *WorkerPool) Dec() {
	w.controlChan <- Sub
}
func (w *WorkerPool) Stop() {
	w.controlChan <- Halt
}

func (w *WorkerPool) Start() {
	count := w.workerCount.Load()
	for range count {
		w.finishedChan <- STUB
	}
	go func() {
		for data := range w.controlChan {
			switch data {
			case Add:
				w.workerCount.Add(1)
				w.finishedChan <- STUB
			case Sub:
				<-w.finishedChan
				w.workerCount.Add(-1)
			case Halt:
				close(w.outChan)
			}
		}
	}()
}

func NewWorkerPool(workerCount int32) *WorkerPool {
	count := atomic.Int32{}
	count.Store(workerCount)
	return &WorkerPool{
		controlChan:  make(chan command, 1024),
		finishedChan: make(chan Stub, 1024),
		workerCount:  &count,
		outChan:      make(chan any, 1024),
	}
}

type task = func() any

type command = int

const (
	Add command = iota
	Sub
	Halt
)
