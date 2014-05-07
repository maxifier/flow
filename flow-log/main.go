package main

import (
	"code.google.com/p/go-uuid/uuid"
	"flag"
	"fmt"
	"github.com/streadway/amqp"
	"log"
	"os"
	"os/signal"
	"time"
)

const (
	version = 1
)

var (
	uri          = flag.String("uri", "amqp://guest:guest@localhost:5672", "AMQP URI")
	exchange     = flag.String("exchange", "mxf.flow", "Durable, non-auto-deleted AMQP exchange name")
	exchangeType = flag.String("exchange-type", "direct", "Exchange type - direct|fanout|topic|x-custom")
	queueName    = flag.String("queue", fmt.Sprintf("flow-%v", version), "Ephemeral AMQP queue name")
	key          = flag.String("key", "mxf", "AMQP binding key")
	tag          = flag.String("tag", fmt.Sprintf("consumer-%s", uuid.New()), "AMQP consumer tag (should not be blank)")

	consumersNumber = flag.Int("n", 1, "Number of parallel consumers")
	path            = flag.String("path", "abc", "Path to log file")
)

func init() {
	flag.Parse()
}

func main() {

	//dial
	conn, err := amqp.Dial(*uri)
	if err != nil {
		log.Fatalf("Dial: %s", err)
	}

	defer conn.Close()

	go func() {
		fmt.Printf("closing: %s", <-conn.NotifyClose(make(chan *amqp.Error)))
	}()

	channel, err := conn.Channel()
	if err != nil {
		log.Fatalf("Channel: %s", err)
	}

	if err = channel.ExchangeDeclare(
		*exchange,     // name of the exchange
		*exchangeType, // type
		true,          // durable
		false,         // delete when complete
		false,         // internal
		false,         // noWait
		nil,           // arguments
	); err != nil {
		log.Fatalf("Exchange Declare: %s", err)
	}

	queue, err := channel.QueueDeclare(
		*queueName, // name of the queue
		true,       // durable
		false,      // delete when usused
		false,      // exclusive
		false,      // noWait
		nil,        // arguments
	)
	if err != nil {
		log.Fatalf("Queue Declare: %s", err)
	}

	if err = channel.QueueBind(
		queue.Name, // name of the queue
		*key,       // bindingKey
		*exchange,  // sourceExchange
		false,      // noWait
		nil,        // arguments
	); err != nil {
		log.Fatalf("Queue Bind: %s", err)
	}

	//close channel used for declartions
	channel.Close()

	//open log file to write
	file, err := os.OpenFile(*path, os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		log.Fatalf("Unable to open log file to write: %s", err)
	}
	defer file.Close()

	consumers := make([]*Consumer, *consumersNumber, *consumersNumber)

	for i := 0; i < len(consumers); i++ {
		ctag := fmt.Sprintf("%s-%d", *tag, i)
		c, err := NewConsumer(conn, &queue, file, ctag)
		log.Printf("Consumer '%s' started\n", ctag)
		if err != nil {
			log.Fatalf("%s", err)
		}
		consumers[i] = c
	}

	signalCh := make(chan os.Signal, 1)
	signal.Notify(signalCh, os.Interrupt)
	<-signalCh
	for _, c := range consumers {
		c.Shutdown()
	}
	log.Println("Bye!")
	os.Exit(0)
}

type Consumer struct {
	conn    *amqp.Connection
	channel *amqp.Channel
	tag     string
	file    *os.File
	done    chan error
	quit    chan bool
}

func NewConsumer(conn *amqp.Connection, queue *amqp.Queue, file *os.File, ctag string) (*Consumer, error) {

	self := &Consumer{
		conn:    conn,
		channel: nil,
		tag:     ctag,
		file:    file,
		done:    make(chan error),
		quit:    make(chan bool),
	}
	var err error
	self.channel, err = self.conn.Channel()
	if err != nil {
		return nil, fmt.Errorf("Channel: %s", err)
	}

	deliveries, err := self.channel.Consume( //<-chan amqp.Delivery
		queue.Name, // name
		self.tag,   // consumerTag,
		true,       // noAck
		false,      // exclusive
		false,      // noLocal
		false,      // noWait
		nil,        // arguments
	)

	go handle(deliveries, self.file, self.done, self.quit)

	return self, nil
}

func (c *Consumer) Shutdown() error {
	//command to stop handling
	c.quit <- true
	// will close() the deliveries channel
	if err := c.channel.Cancel(c.tag, true); err != nil {
		return fmt.Errorf("Consumer cancel failed: %s", err)
	}
	defer log.Printf("Cancel consumer '%s'", c.tag)

	// wait for handle() to exit
	return <-c.done
}

func handle(deliveries <-chan amqp.Delivery, file *os.File, done chan error, quit chan bool) {
	for {
		select {
		case <-quit:
			done <- nil
			return
		case d := <-deliveries:
			file.WriteString(fmt.Sprintf("%v %q;\n", time.Now(), d.Body))

			// log.Printf(
			// 	"got %dB delivery: [%v] %q",
			// 	len(d.Body),
			// 	d.DeliveryTag,
			// 	d.Body,
			// )
			d.Ack(false)
		}
	}
}
