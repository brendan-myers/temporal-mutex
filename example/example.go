package main

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	temporalmutex "github.com/brendan-myers/temporal-mutex"
)

const HOST = "localhost:7233"

func main() {
	ctx := context.Background()

	mutex, err := temporalmutex.NewMutex(ctx, HOST)
	if err != nil {
		log.Fatalln("unable to create mutex", err)
	}
	defer mutex.Close()

	var wg sync.WaitGroup

	for i := range 10 {
		wg.Add(1)

		go func() {
			defer wg.Done()

			fmt.Printf("%d - waiting\n", i)

			err = mutex.Lock(ctx)
			if err != nil {
				log.Fatalf("%d - unable to obtain lock: %v", i, err)
			}

			lctx, cancel := context.WithTimeout(ctx, temporalmutex.START_TO_CLOSE_TIMEOUT)
			defer cancel()

			fmt.Printf("%d - locked\n", i)
			fmt.Printf("%d - critical section\n", i)
			time.Sleep(time.Second * 1)

			if lctx.Err() != nil {
				fmt.Printf("%d - lock timed out\n", i)
				return
			}

			fmt.Printf("%d - releasing lock\n\n", i)

			err = mutex.Unlock(ctx)
			if err != nil {
				log.Fatalf("%d - error releasing lock: %v", i, err)
			}
		}()
	}

	wg.Wait()
}
