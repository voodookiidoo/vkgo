package main

import (
	"context"
	"fmt"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"
)

func main() {
	pool := NewWorkerPool()
	pool.StartHandling()
	pool.Inc()
	pool.Inc()
	pool.Inc()
	for i := range 20 {
		pool.AddTask(func() any {
			time.Sleep(time.Millisecond * time.Duration(rand.Intn(500)+200))
			return fmt.Sprintf("I AM SOME DATA WITH ID %d, INDEX %d", rand.Int()%300, i)
		})
	}
	time.Sleep(time.Second * 4)
	pool.Halt()
	for data := range pool.outchan {
		fmt.Println("recieved data: ", data)
	}

}

type empty struct{}

var STUB = empty{}

type WorkerPool struct {
	taskChan       chan task  // канал по которому таски отправляются горутинам
	finishChan     chan empty // главный канал который завершает работу пула в целом
	killWorkerChan chan empty // канал по которому отправляются запросы за убийство воркера (ужас какой)
	addWorkerChan  chan empty // канал по которому отправляются запросы на добавление таски
	outchan        chan any   // канал данных по которому их отправляют наружу
	workerId       atomic.Int32
	m              sync.Mutex
}

func (p *WorkerPool) Inc() {
	go func() {
		p.addWorkerChan <- STUB
	}()

}
func (p *WorkerPool) Dec() {
	go func() {
		p.killWorkerChan <- STUB
	}()
}
func (p *WorkerPool) Halt() {
	go func() {
		p.finishChan <- STUB
	}()
}

func (p *WorkerPool) AddTask(t task) {
	go func() {
		p.taskChan <- t
	}()
}
func (p *WorkerPool) StartHandling() {
	ctx, cancel := context.WithCancel(context.Background())
	go func(_ctx context.Context) {
		for {
			select {
			case <-p.finishChan:
				cancel()
				close(p.outchan)
				return
			case <-p.addWorkerChan:
				id := p.workerId.Add(1)
				go func(id int32) {
					fmt.Printf("chan with id %d is started\n", id)
					for {
						select {
						case <-p.killWorkerChan:
							fmt.Printf("chan with id %d is finished\n", id)
							break
						case t := <-p.taskChan:
							a := t()
							fmt.Printf("chan with id %d calculated value %s\n", id, a)
							p.outchan <- a
						}

					}
				}(id)

			}
		}

	}(ctx)
}

func NewWorkerPool() *WorkerPool {
	return &WorkerPool{
		finishChan:     make(chan empty, 1024),
		killWorkerChan: make(chan empty, 1024),
		addWorkerChan:  make(chan empty, 1024),
		taskChan:       make(chan task, 1024),
		outchan:        make(chan any, 1024),
		m:              sync.Mutex{},
	}
}

type task = func() any
