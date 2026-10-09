package websocket

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"github.com/pkg/errors"
	"nhooyr.io/websocket"

	"github.com/ez4bk/ezboot"
)

type Server struct {
	cfg *config
	rea *reactor
}

// serveHandler 生成一个Websocket处理函数
func (s *Server) serveHandler(ctx context.Context, handler func(context.Context, *Conn) error) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			w.WriteHeader(http.StatusNotAcceptable)
			return
		}

		select {
		case <-ctx.Done():
			return
		default:
			go func() {
				wrappedConn := &Conn{ctx: context.Background(), underlying: conn}

				connId := s.rea.Attach(wrappedConn)
				wrappedConn.OnClose(func() { s.rea.Detach(wrappedConn) })

				if err := handler(wrappedConn.WithConnContext(ctx), wrappedConn); err != nil {
					s.cfg.Logger.Debugw("failed to handle websocket", "error", err, "connId", connId)
					if re := wrappedConn.Close(); re != nil {
						s.cfg.Logger.Infow("failed to close the websocket", "error", re)
					}
				}
			}()
		}
	}
}

// genWebsocketHttpHandler 生成一个GRPC的处理函数
func (s *Server) genWebsocketHttpHandler(ctx context.Context, gc *grpcClient) runtime.HandlerFunc {
	serveHandler := s.serveHandler(ctx, func(ctx context.Context, conn *Conn) error {
		return s.serveWebsocketConn(ctx, gc, conn)
	})

	return func(w http.ResponseWriter, r *http.Request, _ map[string]string) {
		serveHandler(w, r)
	}
}

// Message 表示一条调用消息
type Message struct {
	Id   *string `json:"id,omitempty" yaml:"id"`
	Kind string  `json:"kind" yaml:"kind"`
	Data any     `json:"data" yaml:"data"`
}

// serveWebsocketConn 处理用户的Websocket请求
func (s *Server) serveWebsocketConn(ctx context.Context, gc *grpcClient, conn *Conn) error {
	errs := make(chan error)
	defer close(errs)

	for {
		msg, err := s.readFromWebsocketConn(ctx, conn, errs)
		if err != nil {
			s.cfg.Logger.Warnw("failed to read message from websocket", "error", err)
			return err
		}

		// process the request
		go func(msg *Message) {
			if resp, err := gc.Call(ctx, msg.Kind, msg.Data); err != nil {
				select {
				case errs <- err:
				default:
				}
			} else if resp != nil {
				respMsg := &Message{Id: msg.Id, Kind: msg.Kind, Data: resp}
				if respData, err := json.Marshal(respMsg); err != nil {
					select {
					case errs <- err:
					default:
					}
				} else {
					if err = conn.Write(ctx, respData); err != nil {
						select {
						case errs <- err:
						default:
						}
					}
				}
			}
		}(msg)
	}
}

// readFromWebsocketConn 从连接中读取消息
func (s *Server) readFromWebsocketConn(ctx context.Context, conn *Conn, errs <-chan error) (*Message, error) {
	keepaliveCtx, keepaliveCancel := context.WithTimeout(ctx, s.cfg.KeepaliveTimeout)
	defer keepaliveCancel()

	errorDetectCtx, errDetectCancel := context.WithCancel(keepaliveCtx)
	defer errDetectCancel()

	var detectedErr error
	go func() {
		select {
		case detectedErr = <-errs:
			errDetectCancel()
		case <-errorDetectCtx.Done():
		}
	}()

	data, err := conn.Read(errorDetectCtx)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			if detectedErr == nil {
				return nil, ErrReadTimeout
			}
			return nil, detectedErr
		}
		return nil, err
	}

	var msg Message
	if err = json.Unmarshal(data, &msg); err != nil {
		return nil, err
	}
	return &msg, nil
}

func newServer() *Server {
	return &Server{
		rea: globalReactor,
		cfg: defaultConfig(),
	}
}

type config struct {
	Logger           *ezboot.Logger
	KeepaliveTimeout time.Duration
}

func defaultConfig() *config {
	return &config{
		Logger:           ezboot.DiscardLogger,
		KeepaliveTimeout: time.Minute,
	}
}
