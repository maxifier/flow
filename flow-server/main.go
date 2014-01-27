package main

import (
	"flag"
	"fmt"
	"github.com/streadway/amqp"
	"log"
	"net/http"
	"os"
	"os/signal"
)

var (
	//http server params
	bind = flag.String("bind", "127.0.0.1:4000", "Address to listen and serve")
	//RabbitMQ params
	uri                = flag.String("rabbitmq.uri", "amqp://guest:guest@localhost:5672", "RabbitMQ URI.")
	exchange           = flag.String("exchange", "mxf.flow", "Exchange to send requests to.")
	exchangeType       = flag.String("type", "direct", "Exchange type")
	exchageDurable     = flag.Bool("durable", true, "Publish exchange as durable.")
	exchangeAutoDelete = flag.Bool("autoDelete", false, "Publish exchange as autoDelete.")
	routingKey         = flag.String("routingKey", "mxf", "Routing key should be used to publish events to exchange.")
)

func init() {
	flag.Parse()
}

type Serve struct {
	flow chan<- *http.Request
}

func (s Serve) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	//fmt.Fprint(w, "Hello!")
	go func(r *http.Request) {
		s.flow <- r
	}(r)
}

func send(flow <-chan *http.Request, quit <-chan bool) error {
	//get connection
	con, err := amqp.Dial(*uri)
	if err != nil {
		return fmt.Errorf("Unable to connect to the bus %s", uri, err)
	}
	defer con.Close()
	//get channel
	ch, err := con.Channel()
	if err != nil {
		return fmt.Errorf("Unable to get Channel: %s", err)
	}
	if err := ch.ExchangeDeclare(
		*exchange,
		*exchangeType,
		*exchageDurable,
		*exchangeAutoDelete,
		false,
		false,
		nil,
	); err != nil {
		return fmt.Errorf("Exchange Declare: %s", err)
	}
	//process requests loop
	for {
		select {
		case r := <-flow:
			//log.Printf("In: %v\n", r.RequestURI)
			if err = ch.Publish(
				*exchange,
				*routingKey,
				true,
				false, //it's not supported nmore since RabbitMQ 3.x
				amqp.Publishing{
					Headers:         amqp.Table{},
					ContentType:     "*/*",
					ContentEncoding: "",
					Body:            []byte(r.RequestURI),
					DeliveryMode:    amqp.Transient,
					Priority:        0,
				},
			); err != nil {
				return fmt.Errorf("Unable to publish: %s", err)
			}
		case <-quit:
			return nil
		}
	}

}

func main() {
	//connect to RabbitMQ and wait for requests
	flow := make(chan *http.Request)
	quit := make(chan bool)
	go send(flow, quit)
	http.Handle("/", &Serve{flow})
	go http.ListenAndServe(*bind, nil)

	signalCh := make(chan os.Signal, 1)
	signal.Notify(signalCh, os.Interrupt)
	<-signalCh
	log.Println("Bye!")
	quit <- true
	os.Exit(0)

}
