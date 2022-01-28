package amqp

import (
	"context"
	"github.com/bee-org/bee/broker"
	"github.com/bee-org/bee/example"
	"os"
	"strconv"
	"sync"
	"testing"
	"time"
)

var (
	ctx = context.Background()
	b   broker.IBroker
	err error
)

func TestMain(m *testing.M) {
	b, err = NewBroker(Config{
		URL:                  os.Getenv("RABBIT_URL"),
		Queue:                "bee",
		RetryMaxReconsume:    3,
		Concurrency:          1,
		DelayedMessagePlugin: true,
	})
	if err != nil {
		panic(err)
	}
	b.Register("print", example.PrintHandler)
	b.Register("sleep", example.SleepHandler)
	b.Register("counter", example.CounterHandler)
	b.Register("error", example.ErrorHandler)
	b.Register("delay", example.DelayHandler)
	//b.Middleware(testFmtCostMw())
	if err = b.Worker(); err != nil {
		panic(err)
	}
	os.Exit(m.Run())
}

func TestPulSarBroker_SendPrint(t *testing.T) {
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

func TestPulSarBroker_SendCounter(t *testing.T) {
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
			if example.CounterResult != tt.wantResult {
				t.Errorf("SendCounter() result = %v, want %v", example.CounterResult, tt.wantResult)
			}
		})
	}
}

func TestPulSarBroker_SendRetry(t *testing.T) {
	type args struct {
		ctx  context.Context
		data string
	}
	tests := []struct {
		name       string
		args       args
		wantErr    bool
		wantResult int
	}{
		{args: args{ctx: ctx, data: "err"}, wantResult: 3},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := b.Send(tt.args.ctx, "error", tt.args.data); (err != nil) != tt.wantErr {
				t.Errorf("Send() error = %v, wantErr %v", err, tt.wantErr)
			}
			time.Sleep(10 * time.Second)
			if example.ErrorCounter[tt.args.data] != tt.wantResult {
				t.Errorf("SendRetry() got = %v, want %v", example.ErrorCounter[tt.args.data], tt.wantResult)
			}
		})
	}
}

func TestPulSarBroker_SendDelay(t *testing.T) {
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
		{args: args{ctx: ctx, delay: 0 * time.Second}},
		{args: args{ctx: ctx, delay: 3 * time.Second}},
		{args: args{ctx: ctx, delay: 5 * time.Second}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			want := time.Now().Add(tt.args.delay)
			if err := b.SendDelay(tt.args.ctx, "delay", tt.args.data, tt.args.delay); (err != nil) != tt.wantErr {
				t.Errorf("SendDelay() error = %v, wantErr %v", err, tt.wantErr)
			}
			got := <-example.DelayResult
			if got.Before(want) || got.Sub(want).Seconds() > 1 {
				t.Errorf("SendDelay() got delay = %v, want %v", got.Second(), want.Second())
			}
		})
	}
}

func TestBroker_Close(t *testing.T) {
	tests := []struct {
		name    string
		wantErr bool
	}{
		{},
		{},
	}
	b := *b.(*Broker)
	b.Broker = broker.NewBroker()
	b.Register("sleep", example.SleepHandler)
	if err = b.Worker(); err != nil {
		panic(err)
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := b.Close(); (err != nil) != tt.wantErr {
				t.Errorf("Close() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestBroker_ReConnect(t *testing.T) {
	for i := 0; i < 100; i++ {
		b.Send(ctx, "print", strconv.Itoa(i))
		time.Sleep(1 * time.Second)
	}
}
