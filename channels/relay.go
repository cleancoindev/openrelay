package channels

import (
	"log"
	"github.com/notegio/openrelay/common"
)

// RelayFilter objects provide a predicate function to determine whether a
// message should be passed to the next stage
type RelayFilter interface {
	Filter(Delivery) bool
}

type IncludeAll struct {
	counter int64
}

func (filter *IncludeAll) Filter(delivery Delivery) bool {
	filter.counter++
	log.Printf("Relayed message : '%v'", filter.counter)
	return true
}

type InvertFilter struct {
	Subfilter RelayFilter
}

func (filter *InvertFilter) Filter(delivery Delivery) bool {
	return !filter.Subfilter.Filter(delivery)
}

type Relay struct {
	consumerChannel ConsumerChannel
	publishers      []Publisher
	filter          RelayFilter
	s               common.Semaphore
}

func (relay *Relay) Start() bool {
	return relay.consumerChannel.StartConsuming()
}

func (relay *Relay) Stop() bool {
	return relay.consumerChannel.StopConsuming()
}

type RelayConsumer struct {
	relay *Relay
}

func (consumer *RelayConsumer) Consume(delivery Delivery) {
	defer func() {
		if r := recover(); r != nil {
			// Something panicked. Return in-flight messages before continuing.
			consumer.relay.consumerChannel.ReturnAllUnacked()
			panic(r)
		}
	}()
	consumer.relay.s.Acquire()
	go func() {
		defer consumer.relay.s.Release()
		if consumer.relay.filter.Filter(delivery) {
			for _, publisher := range consumer.relay.publishers {
				go publisher.Publish(delivery.Payload())
			}
		}
		delivery.Ack()
	}()
}

func NewRelay(channel ConsumerChannel, publishers []Publisher, filter RelayFilter, concurrency int) Relay {
	relay := Relay{
		channel,
		publishers,
		filter,
		make(common.Semaphore, concurrency),
	}
	relay.consumerChannel.AddConsumer(&RelayConsumer{&relay})
	return relay
}
