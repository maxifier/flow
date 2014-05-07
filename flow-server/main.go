package main

import (
	"flag"
	"fmt"
	"github.com/streadway/amqp"
	"log"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"time"
)

var (
	//http server params
	bind = flag.String("bind", "127.0.0.1:4000", "Address to listen and serve")
	//RabbitMQ params
	uri                = flag.String("uri", "amqp://guest:guest@localhost:5672", "RabbitMQ URI.")
	exchange           = flag.String("exchange", "mxf.flow", "Exchange to send requests to.")
	exchangeType       = flag.String("type", "direct", "Exchange type")
	exchageDurable     = flag.Bool("durable", true, "Publish exchange as durable.")
	exchangeAutoDelete = flag.Bool("autoDelete", false, "Publish exchange as autoDelete.")
	routingKey         = flag.String("routingKey", "mxf", "Routing key should be used to publish events to exchange.")
	concurrent         = flag.Int("c", 1, "Number of used processors")
	// behavior params
	verbose = flag.Bool("v", false, "Be verbose.")
)

func init() {
	flag.Parse()
}

type Serve struct {
	flow chan<- *http.Request
}

func (s Serve) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	go func(r *http.Request) {
		s.flow <- r
	}(r)

}

func send(flow <-chan *http.Request) error {
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
			if *verbose {
				log.Printf("In: %v\n", r.RequestURI)
			}
			if err = ch.Publish(
				*exchange,
				*routingKey,
				false, //non-mandatory
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
		}
	}

}

func main() {
	runtime.GOMAXPROCS(*concurrent)
	//connect to RabbitMQ and wait for requests
	flow := make(chan *http.Request)
	//quit := make(chan bool)
	go send(flow)

	//set up http server
	s := &http.Server{
		Addr:         *bind,
		Handler:      &Serve{flow},
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}
	go s.ListenAndServe()

	//catch Ctrl+C
	signalCh := make(chan os.Signal, 1)
	signal.Notify(signalCh, os.Interrupt)
	<-signalCh
	log.Println("Bye!")
	os.Exit(0)

}
