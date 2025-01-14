package common

import (
	"github.com/Shopify/sarama"
	"strings"
	"time"
)

func CheckTopic(addresses string, topic string) error {
	addrs := strings.Split(addresses, ",")
	config := sarama.NewConfig()
	config.Admin.Retry.Max = 2
	config.Net.DialTimeout = 4 * time.Second
	consumer, err := sarama.NewConsumer(addrs, config)
	if err != nil {
		return err
	}
	defer consumer.Close()

	_, err = consumer.Partitions(topic)
	if err != nil {
		return err
	}
	return nil
}

