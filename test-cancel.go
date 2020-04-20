package main

import (
	"context"
	"fmt"
	"sync"
)

func main() {
	ctx, cancelFunc := context.WithCancel(context.Background())
	wg := &sync.WaitGroup{}

	for i := range []int{1,2,3,4} {
		wg.Add(1)
		go worker(ctx, wg, i)
	}

	cancelFunc()
	wg.Wait()

}

func worker(ctx context.Context, wg *sync.WaitGroup, i int) {
	fmt.Printf("started %d\n", i)
	<- ctx.Done()
	fmt.Printf("Stopped %d\n", i)
	wg.Done()
}