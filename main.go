package main

import (
	"blockchain_listener/subscription"
	"errors"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/ethereum/go-ethereum/common"
)

func main() {

	ethSub, err := subscription.New(url())
	if err != nil {
		panic(err)
	}
	defer ethSub.Close()

	exitChan := make(chan os.Signal, 10)
	signal.Notify(exitChan, os.Interrupt, syscall.SIGTERM)

	// checkReconnection(ethSub, 10*time.Second)

	log.Println("wait events")

	ch := make(chan common.Hash, 256)

	for {
		sub, err := ethSub.SubscribePendingTx(ch)

		if err != nil {
			log.Println("failed to subscribe:", err)
			processSubscriptionError(ethSub)
			continue
		}

		exit := waitEvent(ch, sub.Err(), exitChan)
		sub.Unsubscribe()
		if exit {
			log.Println("exit")
			return
		}
	}
}

const URL_ENV_VARIABLE = "ETH_WS_URL"

func url() string {
	url, ok := os.LookupEnv(URL_ENV_VARIABLE)
	if !ok {
		panic("failed to get eth ws url")
	}

	return url
}

func processSubscriptionError(sub *subscription.Subscription) {

	err := sub.Reconnect()
	if errors.Is(err, subscription.ErrClosed) {
		log.Println("trying to reconnect:", err)
	}

	sub.WaitActivation()
}

func waitEvent[T any](eventsCh <-chan T, errorsCh <-chan error, exitCh <-chan os.Signal) (exit bool) {
	for {
		select {
		case event := <-eventsCh:
			log.Printf("%+v", event)
		case err := <-errorsCh:
			log.Println("got error while waiting for event:", err)
			return false
		case <-exitCh:
			return true
		}
	}
}

// func checkReconnection(sub *subscription.Subscription, sleepTime time.Duration) {
// 	go func() {
// 		time.Sleep(sleepTime)

// 		err := sub.Reconnect()
// 		if err != nil {
// 			log.Println("failed to reconnect", err)
// 		}
// 	}()
// }
