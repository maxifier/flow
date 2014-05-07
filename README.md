flow
====

install go
====
http://golang.org/doc/install

flow-server
====
    go get github.com/maxifier/flow/flow-server

    flow-server --help
      Usage of flow-server:
      -autoDelete=false: Publish exchange as autoDelete.
      -bind="127.0.0.1:4000": Address to listen and serve
      -c=1: Number of used processors
      -durable=true: Publish exchange as durable.
      -exchange="mxf.flow": Exchange to send requests to.
      -routingKey="mxf": Routing key should be used to publish events to exchange.
      -type="direct": Exchange type
      -uri="amqp://guest:guest@localhost:5672": RabbitMQ URI.
      -v=false: Be verbose.

flow-log
====
    go get github.com/maxifier/flow/flow-log

    flow-log --help

    Usage of flow-log:
      -exchange="mxf.flow": Durable, non-auto-deleted AMQP exchange name
      -exchange-type="direct": Exchange type - direct|fanout|topic|x-custom
      -key="mxf": AMQP binding key
      -n=1: Number of parallel consumers
      -path="abc": Path to log file
      -queue="flow-1": Ephemeral AMQP queue name
      -tag="consumer-d2a411a4-0903-4b8e-969c-40e9c653aa27": AMQP consumer tag (should not be blank)
      -uri="amqp://guest:guest@localhost:5672": AMQP URI


architecture
====

      http             http             http 
        |                |                |
    flow-server     flow-server     flow-server
         \               |              /
           \             |             /
             \           |            /
               \         |           /
                 \       |          / 
                   \     |         /
                    rabbitmq-server
                    /    |        \
                   /     |         \
                  /      |          \
                 /       |           \      
            flow-log   flow-log    flow-log
               |         |            |
              file      file         file

