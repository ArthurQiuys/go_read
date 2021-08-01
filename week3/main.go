package main

import (
	"context"
	"golang.org/x/sync/errgroup"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	signalChan := make(chan os.Signal, 1)
	stop := make(chan struct{})

	serOne := http.Server{
		Addr: ":8088",
	}

	serTwo := http.Server{
		Addr: ":8089",
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	group, _ := errgroup.WithContext(ctx)

	group.Go(func() error {
		return serOne.ListenAndServe()
	})
	group.Go(func() error {
		return serTwo.ListenAndServe()
	})

	go func() {
		signal.Notify(signalChan, syscall.SIGHUP, syscall.SIGQUIT, syscall.SIGTERM, syscall.SIGINT)
	}()

	go func() {
		for {
			select {
			case <-signalChan:
				log.Println("signal")
				cancel()
			case <-ctx.Done():
				log.Println(ctx.Err())
				serverCancelHandler(stop, &serOne, &serTwo)
			}
		}
	}()

	if err := group.Wait(); err != nil {
		cancel()
		log.Println(err)
	}
	<-stop
}

func serverCancelHandler(stop chan struct{}, servers ...*http.Server) {

	go func() {
		success := 0
		mistake := 0
		for _, server := range servers {
			if err := server.Shutdown(context.Background()); err != nil {
				log.Printf("端口%s shutdown failed, err: %v\n", server.Addr, err)
				mistake++
				continue
			}
			success++
			log.Printf("%s shutdown succrss", server.Addr)
		}

		log.Printf("shutdown completed, success total is %d,fail total is %d", success, mistake)
		close(stop)
		return
	}()

	// 超时强制退出
	<-time.After(time.Minute * 6)
	log.Println("timeout")
	close(stop)
	return
}
