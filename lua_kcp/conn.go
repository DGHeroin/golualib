package lua_kcp

import (
    "encoding/binary"
    "errors"
    "io"
    "net"
    "sync"
    "sync/atomic"
    "time"
)

type Conn struct {
    conn     net.Conn
    id       uint32
    withHead bool
    timeout  time.Duration

    srv               *kcpHandler
    closeChan         chan struct{}
    packetReceiveChan chan []byte
    packetSendChan    chan []byte
    closeFlag         int32
    closeOnce         sync.Once
    openFlag          int32
    callback          ConnCallback
}

type ConnCallback interface {
    OnConnect(conn *Conn) bool
    OnMessage(conn *Conn, pkt []byte) bool
    OnClose(conn *Conn)
}

func (c *Conn) Close() {
    c.closeOnce.Do(func() {
        atomic.StoreInt32(&c.closeFlag, 1)
        close(c.closeChan)
        close(c.packetReceiveChan)
        close(c.packetSendChan)
        c.conn.Close()
        c.callback.OnClose(c)
    })
}

func (c *Conn) Send(payload []byte) (int, error) {
    if c.IsClosed() {
        return -1, errors.New("send on closed conn")
    }
    c.packetSendChan <- payload
    return 0, nil
}
func (c *Conn) ReadMessage() ([]byte, error) {
    conn := c.conn
    var (
        buf  []byte
        data []byte
        n    int
        err  error
    )
    if c.timeout != 0 {
        _ = conn.SetReadDeadline(time.Now().Add(c.timeout))
    }
    if c.withHead {
        buf = make([]byte, 4)
        n, err = io.ReadFull(conn, buf)
        if err != nil {
            return nil, err
        }
        data = buf[:n]
        size := binary.BigEndian.Uint32(data)
        buf = make([]byte, size)
        n, err = io.ReadFull(conn, buf)
        if err != nil {
            return nil, err
        }
        data = buf[:n]
    } else {
        buf = make([]byte, 4096)
        n, err = conn.Read(buf)
        if err != nil {
            return nil, err
        }
        data = buf[:n]
    }
    return data, nil
}

func (c *Conn) WriteMessage(data []byte) {
    if c.withHead {
        header := make([]byte, 4)
        binary.BigEndian.PutUint32(header, uint32(len(data)))
        data = append(header, data...)
    }
    if c.timeout != 0 {
        _ = c.conn.SetWriteDeadline(time.Now().Add(c.timeout))
    }
    if _, err := c.conn.Write(data); err != nil {
        _ = c.conn.Close()
        return
    }
}

func (c *Conn) readLoop() {
    defer func() {
        recover()
        c.Close()
    }()

    for {
        select {
        case <-c.srv.exitChan:
            return
        case <-c.closeChan:
            return
        default:
            data, err := c.ReadMessage()
            if err != nil {
                return
            }
            c.packetReceiveChan <- data
        }
    }
}

func (c *Conn) writeLoop() {
    defer func() {
        recover()
        c.Close()
    }()

    for {
        select {
        case <-c.srv.exitChan:
            return
        case <-c.closeChan:
            return
        case data := <-c.packetSendChan:
            if c.IsClosed() {
                return
            }
            c.conn.SetWriteDeadline(time.Now().Add(c.timeout))
            c.WriteMessage(data)
        }
    }
}

func (c *Conn) IsClosed() bool {
    return atomic.LoadInt32(&c.closeFlag) == 1
}

func asyncDo(fn func(), wg *sync.WaitGroup) {
    wg.Add(1)
    go func() {
        fn()
        wg.Done()
    }()
}
func (c *Conn) SetCallback(callback ConnCallback) {
    c.callback = callback
}
