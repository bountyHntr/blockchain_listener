package subscription

import (
	"context"
	"sync/atomic"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/pkg/errors"
)

var (
	ErrReconnecting = errors.New("subscription is reconnecting")
	ErrClosed       = errors.New("subscription is closed")
)

const (
	contextTimeout = 5 * time.Second

	defaultReconnectInterval = 200 * time.Millisecond
	maxReconnectInterval     = 5 * time.Minute
)

type subscriptionState int32

const (
	active subscriptionState = iota
	reconnecting
	closed
)

// Wrapper for managing subscriptions to blockchain events.
type Subscription struct {
	url   string
	state atomic.Int32
	rpc   *rpc.Client
}

func New(url string) (*Subscription, error) {
	ctx, cancel := context.WithTimeout(context.Background(), contextTimeout)
	defer cancel()

	rpc, err := rpc.DialContext(ctx, url)
	if err != nil {
		return nil, errors.Wrap(err, "failed to dial node")
	}

	s := Subscription{
		url: url,
		rpc: rpc,
	}

	return &s, nil
}

// Restores the connection of underlying RPC client.
func (s *Subscription) Reconnect() error {
	err := s.isActive()
	if err != nil {
		return err
	}

	s.setState(reconnecting)

	go func() {
		var err error
		reconnectInterval := defaultReconnectInterval

		for {
			if s.rpc != nil {
				s.rpc.Close()
			}

			ctx, cancel := context.WithTimeout(context.Background(), contextTimeout)
			s.rpc, err = rpc.DialContext(ctx, s.url)
			cancel()

			if err == nil {
				s.setState(active)
				return
			}

			time.Sleep(reconnectInterval)
			reconnectInterval = Max(2*reconnectInterval, maxReconnectInterval)
		}
	}()

	return nil
}

type VoidType struct{}

var Void = VoidType{}

// Waiting for the RPC client to reconnect.
func (s *Subscription) WaitActivation() chan VoidType {
	ch := make(chan VoidType)

	go func() {
		for {
			err := s.isActive()
			if err == nil {
				ch <- Void
				return
			}

			time.Sleep(defaultReconnectInterval)
		}
	}()

	return ch
}

func (s *Subscription) SubscribeFilterLogs(q ethereum.FilterQuery, ch chan<- types.Log) (ethereum.Subscription, error) {
	arg, err := ethclient.ToFilterArg(q)
	if err != nil {
		return nil, err
	}
	return s.subscribe(ch, "logs", arg)
}

func (s *Subscription) SubscribeNewHeads(ch chan<- *types.Header) (ethereum.Subscription, error) {
	return s.subscribe(ch, "newHeads")
}

func (s *Subscription) SubscribePendingTx(ch chan<- common.Hash) (ethereum.Subscription, error) {
	return s.subscribe(ch, "newPendingTransactions")
}

func (s *Subscription) SubscribeFullPendingTx(ch chan<- *types.Transaction) (ethereum.Subscription, error) {
	return s.subscribe(ch, "newPendingTransactions", true)
}

func (s *Subscription) SubscribeNewBlocks(ch chan<- *types.Block) (ethereum.Subscription, error) {
	return s.subscribe(ch, "newBlocks")
}

func (s *Subscription) Close() {
	if s.isClosed() {
		return
	}

	s.setState(closed)

	if s.rpc != nil {
		s.rpc.Close()
	}
}

func (s *Subscription) subscribe(channel interface{}, args ...interface{}) (ethereum.Subscription, error) {
	err := s.isActive()
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), contextTimeout)
	defer cancel()

	return s.rpc.EthSubscribe(ctx, channel, args...)
}

func (s *Subscription) setState(state subscriptionState) {
	s.state.Store(int32(state))
}

func (s *Subscription) getState() subscriptionState {
	return subscriptionState(s.state.Load())
}

func (s *Subscription) isActive() error {
	switch s.getState() {
	case reconnecting:
		return ErrReconnecting
	case closed:
		return ErrClosed
	}
	return nil
}

func (s *Subscription) isClosed() bool {
	return s.getState() == closed
}
