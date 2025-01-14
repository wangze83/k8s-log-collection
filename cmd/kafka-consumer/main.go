package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	kafka "github.com/segmentio/kafka-go"
)

func consumeMsg() {
	// make a new reader that consumes from topic-A, partition 0, at offset 42
	r := kafka.NewReader(kafka.ReaderConfig{
		Brokers:   kafkaURLs,
		Topic:     topic,
		Partition: 0,
		//MinBytes:  10e3, // 10KB
		MaxBytes: 10e6, // 10MB
		GroupID:  "aaa",
	})
	defer r.Close()

	for {
		m, err := r.ReadMessage(context.Background())
		if err != nil {
			log.Println("error: ", err)
			break
		}

		fmt.Printf("message at offset %d: %s = %s\n", m.Offset, string(m.Key), string(m.Value))
	}
}

/**
kafka-go also supports Kafka consumer groups including broker managed offsets.
To enable consumer groups, simply specify the GroupID in the ReaderConfig.
ReadMessage automatically commits offsets when using consumer groups.
// 会自动提交偏移量
*/
func consumeGroupMsg(groupId string) {
	// make a new reader that consumes from topic-A, partition 0, at offset 42
	r := kafka.NewReader(kafka.ReaderConfig{
		Brokers:   kafkaURLs,
		Topic:     topic,
		Partition: 0,
		GroupID:   groupId,
		//MinBytes:  10e3, // 10KB
		MaxBytes:    10e6, // 10MB
		MaxAttempts: 3,    // 最大重试次数
		MaxWait:     10 * time.Second,
	})

	defer r.Close()

	for {
		m, err := r.ReadMessage(context.Background())
		if err != nil {
			log.Println("error: ", err)
			break
		}

		fmt.Println("current group_id: ", groupId)
		fmt.Printf("message at offset %d: %s = %s\n", m.Offset, string(m.Key), string(m.Value))
	}
}

var (
	kafkaURLs = []string{
		"10.1.1.1:39092",
	}

	topic = "k8s_deployment_yxtest_yxproject-log"
)

func main() {
	// 指定多个消费组模式，消费topic中的消息
	//go consumeGroupMsg("mytest-1")
	go consumeMsg()

	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM, syscall.SIGUSR2, os.Interrupt, syscall.SIGHUP)
	<-ch

	// Create a deadline to wait for.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	<-ctx.Done()
}
