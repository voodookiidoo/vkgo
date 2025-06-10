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
	pool := NewWorkerPool() //создаём пул
	pool.StartHandling()    // запускаем обработчик
	for range 5 {
		pool.Inc() // добавляем пять воркеров
	}
	for i := range 20 {
		pool.AddTask(func() any {
			// запукаем задачи всякие разные
			time.Sleep(time.Millisecond * time.Duration(rand.Intn(300)+300))
			return fmt.Sprintf("data with id %d and index %d", rand.Int()%300, i)
		})
	}
	pool.Dec()
	pool.Dec()
	// удаляем двух воркеров
	time.Sleep(time.Second * 5)
	// ждём чтобы таски повыполнялись
	pool.ShutDown()
	// запускаем завершение пула (всё, из пула теперь только читаем)
	for data := range pool.dataChan { // и читаем теперь данные из выходного канала
		fmt.Println("recieved data: ", data)
	}

}

type WorkerPool struct {
	taskChan       chan task     // канал по которому таски отправляются горутинам
	dataChan       chan any      // канал данных по которому их отправляют наружу
	shutDownChan   chan struct{} // канал завершения, нужен для завершения (логично)
	killWorkerChan chan struct{} // канал по которому отправляются запросы на удаление воркера
	addWorkerChan  chan struct{} // канал по которому отправляются запросы на добавление воркера
	workerId       atomic.Int32  // айдигенератор для воркера
}

func (p *WorkerPool) Inc() {
	go func() {
		p.addWorkerChan <- struct{}{}
	}()

}
func (p *WorkerPool) Dec() {
	go func() {
		p.killWorkerChan <- struct{}{}
	}()
}
func (p *WorkerPool) ShutDown() {
	go func() {
		p.shutDownChan <- struct{}{}
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
			case <-p.shutDownChan:
				cancelFunc()
				wg.Wait()
				close(p.shutDownChan)
				close(p.killWorkerChan)
				close(p.addWorkerChan)
				close(p.dataChan)
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
								p.dataChan <- a
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
		shutDownChan:   make(chan struct{}),
		killWorkerChan: make(chan struct{}),
		addWorkerChan:  make(chan struct{}),
		taskChan:       make(chan task),
		dataChan:       make(chan any, 1024),
	}
}

type task = func() any
