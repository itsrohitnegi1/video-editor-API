package receiver

import (
	"context"
	"encoding/json"
	"log/slog"

	"github.com/douglasdgoulart/video-editor-api/pkg/configuration"
	"github.com/douglasdgoulart/video-editor-api/pkg/event"
	"github.com/twmb/franz-go/pkg/kgo"
)

type KafkaEventReceiver struct {
	cl     event.KgoClient
	logger *slog.Logger
}

func NewKafkaEventReceiver(cfg *configuration.Configuration) EventReceiver {
	kafkaConsumerConfig := cfg.Kafka.KafkaConsumerConfig
	logger := cfg.Logger.WithGroup("kafka-event-receiver")
	getKgoOffset(kafkaConsumerConfig)

	cl, err := kgo.NewClient(
		kgo.SeedBrokers(kafkaConsumerConfig.Brokers...),
		kgo.ConsumerGroup(kafkaConsumerConfig.GroupID),
		kgo.ConsumeTopics(kafkaConsumerConfig.Topic),
	)
	if err != nil {
		panic(err)
	}
	return &KafkaEventReceiver{
		cl:     cl,
		logger: logger,
	}
}

func getKgoOffset(cfg configuration.KafkaConsumerConfig) kgo.Offset {
	if cfg.Offset == "earliest" {
		return kgo.NewOffset().AtStart()
	}

	return kgo.NewOffset().AtEnd()
}

func (k *KafkaEventReceiver) Receive(ctx context.Context, handle func(event *event.Event) error) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
			fetches := k.cl.PollFetches(ctx)
			iter := fetches.RecordIter()
			for !iter.Done() {
				var e event.Event
				record := iter.Next()

				if err := json.Unmarshal(record.Value, &e); err != nil {
					k.logger.Error("error unmarshalling event", "error", err, "event", string(record.Value))
					continue
				}
				k.logger.Debug("received event", "event", e)

				var processErr error
				if processErr = handle(&e); processErr != nil {
					k.logger.Error("error handling event", "error", processErr)
				}
			}
		}
	}
}
