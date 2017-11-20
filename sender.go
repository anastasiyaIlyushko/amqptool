package amqptool

import (
	"fmt"

	"github.com/streadway/amqp"
)

type message struct {
	RoutingKey string
	Body       []byte
}

// Sender contains data for sending messages.
type Sender struct {
	Cnn             string
	Exchange        string
	ExchangeDeclare bool
	ExchangeOpt     ExchangeOpt
	Err             chan error
	inC             chan message
}

// NewSender create & init the Sender struct.
func NewSender(cnn, exchange string) *Sender {
	return &Sender{
		Cnn:             cnn,
		Exchange:        exchange,
		ExchangeDeclare: true,
		ExchangeOpt: ExchangeOpt{
			Durable: true,
			Kind:    "direct",
		},
		Err: make(chan error),
		inC: make(chan message),
	}
}

// Connect creates a connection to the amqp-server and starts the message sending loop.
func (s *Sender) Connect() error {

	conn, err := amqp.Dial(s.Cnn)
	if err != nil {
		return err
	}

	ch, err := conn.Channel()
	if err != nil {
		conn.Close()
		return err
	}

	if s.ExchangeDeclare {
		if err := ch.ExchangeDeclare(
			s.Exchange,
			s.ExchangeOpt.Kind,
			s.ExchangeOpt.Durable,
			s.ExchangeOpt.Autodelete,
			s.ExchangeOpt.Internal,
			s.ExchangeOpt.NoWait,
			amqp.Table(s.ExchangeOpt.Args),
		); err != nil {
			ch.Close()
			conn.Close()
			return err
		}
	}

	go s.start(conn, ch)

	return nil
}

// Send sends a message to the exchange with a certain key.
func (s *Sender) Send(body []byte, routingKey string) {
	msg := message{
		RoutingKey: routingKey,
		Body:       body,
	}
	s.inC <- msg
}

func (s *Sender) start(conn *amqp.Connection, ch *amqp.Channel) {

	defer conn.Close()
	defer ch.Close()

	forever := make(chan error)

	go func() {
		for msg := range s.inC {
			if err := ch.Publish(
				s.Exchange,
				msg.RoutingKey,
				false,
				false,
				amqp.Publishing{
					ContentType: "text/plain",
					Body:        msg.Body,
				},
			); err != nil {
				forever <- err
				return
			}
		}
		forever <- nil
	}()

	select {
	case err := <-ch.NotifyClose(make(chan *amqp.Error)):
		s.Err <- fmt.Errorf("NotifyClose %s", err.Error())
	case err := <-forever:
		if err != nil {
			s.Err <- err
		}
	}
}
