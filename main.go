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
	for range 5 {
		pool.Inc()
	}
	for i := range 20 {
		pool.AddTask(func() any {
			time.Sleep(time.Millisecond * time.Duration(rand.Intn(500)+200))
			return fmt.Sprintf("data with id %d and index %d", rand.Int()%300, i)
		})
	}
	pool.Dec()
	pool.Dec()
	time.Sleep(time.Second * 4)
	pool.ShutDown()
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
func (p *WorkerPool) ShutDown() {
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
	wg := sync.WaitGroup{}
	go func(ctx context.Context, cancelFunc context.CancelFunc) {
		for {
			select {
			case <-p.finishChan:
				cancelFunc()
				wg.Wait()
				close(p.finishChan)
				close(p.killWorkerChan)
				close(p.addWorkerChan)
				close(p.outchan)
				close(p.taskChan)
				return
			case <-p.addWorkerChan:
				wg.Add(1)
				id := p.workerId.Add(1)
				go func(id int32) {
					defer wg.Done()
					fmt.Printf("chan with id %d is started\n", id)
					for {
						select {
						case <-ctx.Done():
							return
						case <-p.killWorkerChan:
							fmt.Printf("chan with id %d is finished\n", id)
							break
						case t := <-p.taskChan:
							a := t()
							select {
							case <-ctx.Done():
								return
							default:
								fmt.Printf("chan with id %d calculated value %s\n", id, a)
								p.outchan <- a
							}
						}

					}
				}(id)
			}
		}
	}(ctx, cancel)
}

func NewWorkerPool() *WorkerPool {
	return &WorkerPool{
		finishChan:     make(chan empty),
		killWorkerChan: make(chan empty),
		addWorkerChan:  make(chan empty),
		taskChan:       make(chan task),
		outchan:        make(chan any, 1024),
	}
}

type task = func() any
