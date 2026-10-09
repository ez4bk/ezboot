package middleware

import (
	"errors"
	"sync"
	"time"

	conf_type "github.com/ez4bk/ezboot/internal/gin-boot/conf-type"
	"github.com/ez4bk/ezboot/internal/logging"
	amqp "github.com/rabbitmq/amqp091-go"
)

type RabbitMQClient struct {
	Config     *conf_type.RabbitMQ
	Publishers map[string]*RabbitMQPublisher
	Consumers  map[string]*RabbitMQConsumer
}

func (r *RabbitMQClient) Shutdown() {
	for _, publisher := range r.Publishers {
		_ = publisher.Close()
	}
	for _, consumer := range r.Consumers {
		_ = consumer.Close()
	}
}

func (mw *Middleware) initRabbitMQ() {
	conf := &mw.conf.App.Middleware.RabbitMQ
	if !conf.Startup {
		return
	}

	client := &RabbitMQClient{
		Config:     conf,
		Publishers: make(map[string]*RabbitMQPublisher),
		Consumers:  make(map[string]*RabbitMQConsumer),
	}

	for _, publisher := range conf.Publishers {
		client.Publishers[publisher.Name] = NewRabbitMQPublisher(mw.logger, conf.Url, publisher.Exchange)
	}

	for _, consumer := range conf.Consumers {
		client.Consumers[consumer.Name] = NewRabbitMQConsumer(mw.logger, conf.Url, consumer.Exchange, consumer.Queue)
	}

	mw.rabbitMQ = client
}

const (
	reconnectDelay = 5 * time.Second
	reInitDelay    = 2 * time.Second
	resendDelay    = 5 * time.Second
)

var (
	errNotConnected  = errors.New("not connected to a server")
	errAlreadyClosed = errors.New("already closed: not connected to the server")
	errShutdown      = errors.New("client is shutting down")
)

type channelInitializer interface {
	initChannel(ch *amqp.Channel) error
}

type RabbitMQBase struct {
	url string

	mu              sync.Mutex
	logger          *logging.Logger
	connection      *amqp.Connection
	channel         *amqp.Channel
	done            chan bool
	notifyConnClose chan *amqp.Error
	notifyChanClose chan *amqp.Error
	isReady         bool

	initializer channelInitializer
}

func newRabbitMQBase(l *logging.Logger, url string, init channelInitializer) RabbitMQBase {
	return RabbitMQBase{
		logger:      l,
		url:         url,
		done:        make(chan bool),
		initializer: init,
	}
}

func (b *RabbitMQBase) Start() {
	go b.handleReconnect()
}

// handleReconnect will wait for a connection error on
// notifyConnClose, and then continuously attempt to reconnect.
func (b *RabbitMQBase) handleReconnect() {
	for {
		b.mu.Lock()
		b.isReady = false
		b.mu.Unlock()

		b.logger.Info("RabbitMQ: attempting to connect")

		conn, err := b.connect(b.url)

		if err != nil {
			b.logger.Error("RabbitMQ: connect failed, retrying...")

			select {
			case <-b.done:
				return
			case <-time.After(reconnectDelay):
			}
			continue
		}

		if done := b.handleReInit(conn); done {
			return
		}
	}
}

// connect will create a new AMQP connection
func (b *RabbitMQBase) connect(addr string) (*amqp.Connection, error) {
	conn, err := amqp.Dial(addr)

	if err != nil {
		return nil, err
	}

	b.changeConnection(conn)
	b.logger.Info("RabbitMQ: connected")
	return conn, nil
}

func (b *RabbitMQBase) handleReInit(conn *amqp.Connection) bool {
	for {
		b.mu.Lock()
		b.isReady = false
		b.mu.Unlock()

		err := b.init(conn)

		if err != nil {
			b.logger.Error("RabbitMQ: channel init failed, retrying...")

			select {
			case <-b.done:
				return true
			case <-b.notifyConnClose:
				b.logger.Error("RabbitMQ: connection closed, reconnecting...")
				return false
			case <-time.After(reInitDelay):
			}
			continue
		}

		select {
		case <-b.done:
			return true
		case <-b.notifyConnClose:
			b.logger.Error("RabbitMQ: connection closed, reconnecting...")
			return false
		case <-b.notifyChanClose:
			b.logger.Error("RabbitMQ: channel closed, re-initializing...")
		}
	}
}

func (b *RabbitMQBase) init(conn *amqp.Connection) error {
	ch, err := conn.Channel()
	if err != nil {
		return err
	}

	if err = b.initializer.initChannel(ch); err != nil {
		_ = ch.Close()
		return err
	}

	b.changeChannel(ch)
	b.mu.Lock()
	b.isReady = true
	b.mu.Unlock()
	b.logger.Info("RabbitMQ: channel ready")
	return nil
}

// changeConnection takes a new connection to the queue,
// and updates the close listener to reflect this.
func (b *RabbitMQBase) changeConnection(connection *amqp.Connection) {
	b.connection = connection
	b.notifyConnClose = make(chan *amqp.Error, 1)
	b.connection.NotifyClose(b.notifyConnClose)
}

// changeChannel takes a new channel to the queue,
// and updates the channel listeners to reflect this.
func (b *RabbitMQBase) changeChannel(channel *amqp.Channel) {
	b.channel = channel
	b.notifyChanClose = make(chan *amqp.Error, 1)
	b.channel.NotifyClose(b.notifyChanClose)
}

func (b *RabbitMQBase) GetChannel() *amqp.Channel {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.channel
}

// Close will cleanly shut down the channel and connection.
func (b *RabbitMQBase) Close() error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if !b.isReady {
		return errAlreadyClosed
	}
	close(b.done)
	if err := b.channel.Close(); err != nil {
		return err
	}
	if err := b.connection.Close(); err != nil {
		return err
	}
	b.isReady = false
	return nil
}

type RabbitMQPublisher struct {
	RabbitMQBase
	exchange      *conf_type.Exchange
	notifyConfirm chan amqp.Confirmation
}

func NewRabbitMQPublisher(l *logging.Logger, url string, exchange *conf_type.Exchange) *RabbitMQPublisher {
	p := &RabbitMQPublisher{exchange: exchange}
	p.RabbitMQBase = newRabbitMQBase(l, url, p)
	p.Start()
	return p
}

// initChannel updates the channel listeners
func (p *RabbitMQPublisher) initChannel(ch *amqp.Channel) error {
	if err := ch.ExchangeDeclare(
		p.exchange.Name,       // name
		p.exchange.Type,       // type
		p.exchange.Durable,    // durable
		p.exchange.AutoDelete, // auto-deleted
		p.exchange.Internal,   // internal
		p.exchange.NoWait,     // no-wait
		nil,                   // arguments
	); err != nil {
		return err
	}

	if err := ch.Confirm(false); err != nil {
		return err
	}
	p.notifyConfirm = make(chan amqp.Confirmation, 1)
	ch.NotifyPublish(p.notifyConfirm)
	return nil
}

// Push will push data onto the queue, and wait for a confirmation.
// This will block until the server sends a confirmation. Errors are
// only returned if the push action itself fails, see UnsafePush.
func (p *RabbitMQPublisher) Push(routingKey string, data []byte) error {
	p.mu.Lock()
	if !p.isReady {
		p.mu.Unlock()
		return errors.New("publisher not ready")
	}
	p.mu.Unlock()

	for {
		err := p.UnsafePush(routingKey, data)
		if err != nil {
			p.logger.Error("Push failed. Retrying...")
			select {
			case <-p.done:
				return errShutdown
			case <-time.After(resendDelay):
			}
			continue
		}
		confirm := <-p.notifyConfirm
		if confirm.Ack {
			p.logger.Info("Push confirmed [%d]!", confirm.DeliveryTag)
			return nil
		}
	}
}

// UnsafePush will push to the queue without checking for
// confirmation. It returns an error if it fails to connect.
// No guarantees are provided for whether the server will
// receive the message.
func (p *RabbitMQPublisher) UnsafePush(routingKey string, data []byte) error {
	p.mu.Lock()
	if !p.isReady {
		p.mu.Unlock()
		return errNotConnected
	}
	p.mu.Unlock()

	return p.channel.Publish(
		p.exchange.Name,
		routingKey,
		false,
		false,
		amqp.Publishing{
			ContentType: "text/plain",
			Body:        data,
		},
	)
}

type RabbitMQConsumer struct {
	RabbitMQBase
	exchange *conf_type.Exchange
	queue    *conf_type.Queue
}

func NewRabbitMQConsumer(l *logging.Logger, url string, exchange *conf_type.Exchange, queue *conf_type.Queue) *RabbitMQConsumer {
	c := &RabbitMQConsumer{queue: queue, exchange: exchange}
	c.RabbitMQBase = newRabbitMQBase(l, url, c)
	c.Start()
	return c
}

// initChannel declare the queues, exchanges and bindings
func (c *RabbitMQConsumer) initChannel(ch *amqp.Channel) error {
	if err := ch.ExchangeDeclare(
		c.exchange.Name,       // name
		c.exchange.Type,       // type
		c.exchange.Durable,    // durable
		c.exchange.AutoDelete, // auto-deleted
		c.exchange.Internal,   // internal
		c.exchange.NoWait,     // no-wait
		nil,                   // arguments
	); err != nil {
		return err
	}

	_, err := ch.QueueDeclare(
		c.queue.Name,
		c.queue.Durable,
		c.queue.AutoDelete,
		c.queue.Exclusive,
		c.queue.NoWait,
		nil,
	)
	if err != nil {
		return err
	}

	if c.queue.Bind.Exchange != "" {
		err = ch.QueueBind(
			c.queue.Name,
			c.queue.Bind.BindingKey,
			c.queue.Bind.Exchange,
			c.queue.Bind.NoWait,
			nil,
		)
		if err != nil {
			return err
		}
	}
	return nil
}

// Consume will continuously put queue items on the channel.
// It is required to call delivery.Ack when it has been
// successfully processed, or delivery.Nack when it fails.
// Ignoring this will cause data to build up on the server.
func (c *RabbitMQConsumer) Consume(prefetchCount int) (<-chan amqp.Delivery, error) {
	c.mu.Lock()
	if !c.isReady {
		c.mu.Unlock()
		return nil, errNotConnected
	}
	c.mu.Unlock()

	if err := c.channel.Qos(prefetchCount, 0, false); err != nil {
		return nil, err
	}

	return c.channel.Consume(
		c.queue.Name,
		"",
		false,
		false,
		false,
		false,
		nil,
	)
}
