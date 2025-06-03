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

	for i := range 100 {
		wg.Add(1)

		go func() {
			defer wg.Done()

			fmt.Printf("%d - waiting\n", i)

			_, err = mutex.Lock(ctx)
			if err != nil {
				log.Fatalf("%d - unable to obtain lock: %v", i, err)
			}

			fmt.Printf("%d - locked\n", i)
			fmt.Printf("%d - critical section\n", i)
			time.Sleep(time.Second * 1)

			fmt.Printf("%d - releasing lock\n\n", i)
			err = mutex.Unlock(ctx)
			if err != nil {
				log.Fatalf("%d - error releasing lock: %v", i, err)
			}
		}()
	}

	wg.Wait()
}
