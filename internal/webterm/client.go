package webterm

import (
	"encoding/base64"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

type MessageType string

const (
	MsgTypeData   MessageType = "data"
	MsgTypeResize MessageType = "resize"
	MsgTypePing   MessageType = "ping"
	MsgTypePong   MessageType = "pong"
	MsgTypeAgent  MessageType = "agent"
	MsgTypeReset  MessageType = "reset"
	MsgTypeClose  MessageType = "disconnect"
)

type Message struct {
	Type MessageType `json:"type"`
	Data string      `json:"data,omitempty"`
	Rows uint16      `json:"rows,omitempty"`
	Cols uint16      `json:"cols,omitempty"`
}

type Client struct {
	id        string
	conn      *websocket.Conn
	server    *Server
	sendCh    chan Message
	closeCh   chan struct{}
	closeOnce sync.Once
	addr      string
	userAgent string
}

func NewClient(id string, conn *websocket.Conn, server *Server, addr string, userAgent string) *Client {
	c := &Client{
		id:        id,
		conn:      conn,
		server:    server,
		sendCh:    make(chan Message, 256),
		closeCh:   make(chan struct{}),
		addr:      addr,
		userAgent: userAgent,
	}

	go c.readLoop()
	go c.writeLoop()

	return c
}

func (c *Client) readLoop() {
	defer c.Close()

	for {
		var msg Message
		err := c.conn.ReadJSON(&msg)
		if err != nil {
			return
		}

		switch msg.Type {
		case MsgTypeData:
			// Mark web as active when receiving input
			c.server.setActiveSource("web")

			data, err := base64.StdEncoding.DecodeString(msg.Data)
			if err != nil {
				continue
			}
			if c.server.proxy != nil {
				_, _ = c.server.proxy.Write(data)
			}

		case MsgTypeResize:
			// if c.server.proxy != nil {
			// 	_ = c.server.proxy.Resize(msg.Rows, msg.Cols)
			// }

		case MsgTypePing:
			_ = c.conn.WriteJSON(Message{Type: MsgTypePong})
		}
	}
}

func (c *Client) writeLoop() {
	defer c.Close()

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case msg := <-c.sendCh:
			if err := c.conn.WriteJSON(msg); err != nil {
				return
			}

		case <-ticker.C:
			if err := c.conn.WriteJSON(Message{Type: MsgTypePing}); err != nil {
				return
			}

		case <-c.closeCh:
			return
		}
	}
}

func (c *Client) Send(data []byte) {
	msg := Message{
		Type: MsgTypeData,
		Data: base64.StdEncoding.EncodeToString(data),
	}
	c.SendMessage(msg)
}

func (c *Client) SendAgent(name string) {
	msg := Message{
		Type: MsgTypeAgent,
		Data: name,
	}
	c.SendMessage(msg)
}

func (c *Client) SendReset() {
	c.SendMessage(Message{Type: MsgTypeReset})
}

func (c *Client) SendDisconnect(reason string) {
	msg := Message{
		Type: MsgTypeClose,
		Data: reason,
	}
	c.SendMessage(msg)
}

func (c *Client) SendMessage(msg Message) {
	select {
	case c.sendCh <- msg:
	case <-c.closeCh:
	default:
		// Drop if buffer full
	}
}

func (c *Client) Close() {
	c.closeOnce.Do(func() {
		close(c.closeCh)
		_ = c.conn.Close()
		c.server.removeClient(c.id)
	})
}

func (c *Client) CloseWithReason(code int, reason string) {
	c.closeOnce.Do(func() {
		_ = c.conn.WriteControl(
			websocket.CloseMessage,
			websocket.FormatCloseMessage(code, reason),
			time.Now().Add(1*time.Second),
		)
		close(c.closeCh)
		_ = c.conn.Close()
		c.server.removeClient(c.id)
	})
}

func (c *Client) Info() ClientInfo {
	return ClientInfo{
		ID:        c.id,
		Addr:      c.addr,
		UserAgent: c.userAgent,
	}
}
