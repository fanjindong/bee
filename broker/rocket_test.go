package broker

import (
	"context"
	"os"
	"strings"
	"sync"
	"testing"
	"time"
)

func initRocketMQBroker() IBroker {
	b, err := NewRocketMQBroker(RocketMQConfig{
		Hosts:             strings.Split(os.Getenv("ROCKETMQ_URL"), ","),
		Topic:             "BEE",
		ProducerGroupName: "BEE-producer",
		ConsumerGroupName: "BEE-consumer",
		Order:             false,
		BroadCasting:      false,
	})
	if err != nil {
		panic(err)
	}
	b.Register("print", printHandler)
	b.Register("sleep", sleepHandler)
	b.Register("counter", counterHandler)
	b.Register("error", errorHandler)
	b.Register("delay", delayHandler)
	//b.Middleware(testFmtCostMw())
	if err = b.Worker(); err != nil {
		panic(err)
	}
	return b
}

func TestRocketMQBroker_SendPrint(t *testing.T) {
	type args struct {
		ctx  context.Context
		name string
		data interface{}
	}
	tests := []struct {
		name       string
		args       args
		wantErr    bool
		wantResult []string
	}{
		{args: args{ctx: ctx, name: "print", data: "a"}, wantResult: []string{"a"}},
		{args: args{ctx: ctx, name: "print", data: "b"}, wantResult: []string{"a", "b"}},
		{args: args{ctx: ctx, name: "print", data: "c"}, wantResult: []string{"a", "b", "c"}},
	}
	b := initRocketMQBroker()
	defer func() { _ = b.Close() }()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := b.Send(tt.args.ctx, tt.args.name, tt.args.data); (err != nil) != tt.wantErr {
				t.Errorf("Send() error = %v, wantErr %v", err, tt.wantErr)
			}
			time.Sleep(1 * time.Second)
			want := make(map[string]struct{})
			for _, v := range tt.wantResult {
				want[v] = struct{}{}
			}
		})
	}
}

func TestRocketMQBroker_SendCounter(t *testing.T) {
	type args struct {
		ctx   context.Context
		batch int
	}
	tests := []struct {
		name       string
		args       args
		wantErr    bool
		wantResult int64
	}{
		{args: args{ctx: ctx, batch: 1}, wantResult: 1},
		{args: args{ctx: ctx, batch: 10}, wantResult: 11},
		{args: args{ctx: ctx, batch: 100}, wantResult: 111},
	}
	b := initRocketMQBroker()
	defer func() { _ = b.Close() }()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wg := sync.WaitGroup{}
			for i := 0; i < tt.args.batch; i++ {
				wg.Add(1)
				go func() {
					if err := b.Send(tt.args.ctx, "counter", nil); (err != nil) != tt.wantErr {
						t.Errorf("SendCounter() error = %v, wantErr %v", err, tt.wantErr)
					}
					wg.Done()
				}()
			}
			wg.Wait()
			time.Sleep(time.Duration(tt.args.batch) * 100 * time.Millisecond)
			if counterResult != tt.wantResult {
				t.Errorf("SendCounter() result = %v, want %v", counterResult, tt.wantResult)
			}
		})
	}
}

func TestRocketMQBroker_SendDelay(t *testing.T) {
	type args struct {
		ctx   context.Context
		data  interface{}
		delay time.Duration
	}
	tests := []struct {
		name       string
		args       args
		wantErr    bool
		wantResult []string
	}{
		{args: args{ctx: ctx, delay: 1 * time.Second}},
		//{args: args{ctx: ctx, delay: 3 * time.Second}},
		//{args: args{ctx: ctx, delay: 5 * time.Second}},
	}
	b := initRocketMQBroker()
	defer func() { _ = b.Close() }()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			want := time.Now().Add(tt.args.delay)
			if err := b.SendDelay(tt.args.ctx, "delay", tt.args.data, tt.args.delay); (err != nil) != tt.wantErr {
				t.Errorf("SendDelay() error = %v, wantErr %v", err, tt.wantErr)
			}
			got := <-delayResult
			if got.Before(want) || got.Sub(want).Seconds() >= 1 {
				t.Errorf("SendDelay() got delay = %v, want %v", got.Second(), want.Second())
			}
		})
	}
}
