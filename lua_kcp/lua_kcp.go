package lua_kcp

import (
    "container/list"
    "encoding/binary"
    "io"
    "log"
    "net"
    "sync"
    "sync/atomic"
    "time"

    "github.com/DGHeroin/golua/lua"
    . "github.com/DGHeroin/golualib"

    "github.com/xtaci/kcp-go"
)

var (
    initCode = `
local lib = lua_kcp
lua_http = nil

function KCPServer()
    local self = {}
    local handler

    local function onEvent( evtType, id, client, msgType, msgData )
        if self.onEvent then
            self.onEvent(evtType, id, client, msgType, msgData)
        end
    end

    function self.Init(addr)
        handler = lib.listen( addr, onEvent )
    end

    function self.Close(client)
        if not handler then return end
        lib.close( client )
    end
    
    function self.Send(client, t, d)
        if not handler then return end
        lib.send(client, t, d)
    end

    function self.SetHead(client, b)
        lib.setHead(client, b)
    end

    function self.SetTimeout(client, sec)
        lib.setTimeout(client, sec)
    end

    return self
end

`
)

func Register(L *lua.State) {
    L.CreateTable(0, 1)

    //  server
    L.PushString("listen")
    L.PushGoFunction(listenServer)
    L.SetTable(-3)

    L.PushString("close")
    L.PushGoFunction(closeConn)
    L.SetTable(-3)

    L.PushString("send")
    L.PushGoFunction(sendConn)
    L.SetTable(-3)

    L.PushString("setHead")
    L.PushGoFunction(setHead)
    L.SetTable(-3)

    L.PushString("setTimeout")
    L.PushGoFunction(setTimeout)
    L.SetTable(-3)

    // everything done
    L.SetGlobal("lua_kcp")

    err := L.DoString(initCode)
    if err != nil {
        log.Println(err)
    }
}

type kcpHandler struct {
    ctx LuaContext
    L   *lua.State
    ref int
    id  uint32
}

func listenServer(L *lua.State) int {
    addr := L.CheckString(1)
    ref := L.Ref(lua.LUA_REGISTRYINDEX)

    ctx := CheckLuaContext(L)
    handler := &kcpHandler{
        ctx: ctx,
        L:   L,
        ref: ref,
    }

    go func() {
        defer func() {
            L.Unref(lua.LUA_REGISTRYINDEX, ref)
        }()
        ln, err := kcp.Listen(addr)
        if err != nil {
            log.Println(err)
        }

        for {
            conn, err := ln.Accept()
            if err != nil {
                log.Println(err)
                return
            }
            go handlerFunc(handler, conn)
        }

    }()

    L.PushGoStruct(handler)
    return 1
}

type kcpClient struct {
    close     func()
    sendMutex sync.Mutex
    sendList  list.List
    send      func(payload []byte) (int, error)
    id        uint32
    withHead  bool
    timeout   time.Duration
}

func handlerFunc(h *kcpHandler, conn net.Conn) {
    var (
        L          = h.L
        ref        = h.ref
        wg         sync.WaitGroup
        client     = &kcpClient{}
        ctx        = h.ctx
        maxPending = 10
        writeSig   = make(chan interface{}, maxPending)
        isRunning  = true
    )

    client.id = atomic.AddUint32(&h.id, 1)
    client.close = func() {
        conn.Close()
    }
    client.send = func(payload []byte) (int, error) {
        client.sendMutex.Lock()
        defer client.sendMutex.Unlock()

        if client.sendList.Len() >= maxPending {
            log.Println("too many messages pending")
            conn.Close()
            return 0, nil
        }
        client.sendList.PushBack(payload)
        writeSig <- nil
        return 0, nil
    }

    defer func() {
        conn.Close()
        // 通知关闭
        ctx.Run(func() {
            L.RawGeti(lua.LUA_REGISTRYINDEX, ref)
            L.PushInteger(3)
            L.PushInteger(int64(client.id))
            L.PushGoStruct(client)
            if err := L.Call(3, 0); err != nil {
                log.Println(err)
            }
        })
    }()
    // 通知新连接
    var wgAccept sync.WaitGroup
    wgAccept.Add(1)
    ctx.Run(func() {
        L.RawGeti(lua.LUA_REGISTRYINDEX, ref)
        L.PushInteger(1)
        L.PushInteger(int64(client.id))
        L.PushGoStruct(client)
        if err := L.Call(3, 0); err != nil {
            log.Println(err)
        }
        wgAccept.Done()
    })
    wgAccept.Wait()
    //log.Println("回调完成, 开始读写", client.withHead, client.timeout)
    // read
    wg.Add(1)
    go func() {
        defer func() {
            isRunning = false
            writeSig<-nil // wait up write thread
            wg.Done()
        }()
        var (
            buf  []byte
            data []byte
            n    int
            err  error
        )
        for isRunning {
            if client.timeout != 0 {
                _ = conn.SetReadDeadline(time.Now().Add(client.timeout))
            }
            if client.withHead {
                buf = make([]byte, 4)
                n, err = io.ReadFull(conn, buf)
                if err != nil {
                    //log.Println(err)
                    return
                }
                data = buf[:n]
                size := binary.BigEndian.Uint32(data)
                buf = make([]byte, size)
                n, err = io.ReadFull(conn, buf)
                if err != nil {
                    //log.Println(err)
                    return
                }
                data = buf[:n]
            } else {
                buf = make([]byte, 4096)
                n, err := conn.Read(buf)
                if err != nil {
                    //log.Println(err)
                    return
                }
                data = buf[:n]
            }
            var wgLua sync.WaitGroup
            wgLua.Add(1)
            ctx.Run(func() {
                L.RawGeti(lua.LUA_REGISTRYINDEX, ref)
                L.PushInteger(2)
                L.PushInteger(int64(client.id))
                L.PushGoStruct(client)
                L.PushInteger(0)
                if data == nil || len(data) == 0 {
                    L.PushNil()
                } else {
                    L.PushBytes(data)
                }

                if err := L.Call(5, 0); err != nil {
                    log.Println(err)
                }
                wgLua.Done()
            })

        }
    }()
    // write
    wg.Add(1)
    go func() {
        defer func() {
            wg.Done()
        }()
        for isRunning {
            select {
            case <-writeSig:
                if !isRunning {
                    return
                }
                var data []byte
                client.sendMutex.Lock()
                if client.sendList.Len() > 0 {
                    data = client.sendList.Remove(client.sendList.Front()).([]byte)
                }
                client.sendMutex.Unlock()
                if client.withHead {
                    header := make([]byte, 4)
                    binary.BigEndian.PutUint32(header, uint32(len(data)))
                    data = append(header, data...)
                }
                if client.timeout != 0 {
                    _ = conn.SetWriteDeadline(time.Now().Add(client.timeout))
                }
                if _, err := conn.Write(data); err != nil {
                    _ = conn.Close()
                    return
                }
            }
        }
    }()
    wg.Wait()
}

func closeConn(L *lua.State) int {
    p := L.ToGoStruct(1)
    if client, ok := p.(*kcpClient); ok {
        client.close()
    } else {
        log.Println("转换 kcp-client 对象失败")
    }
    return 0
}

func sendConn(L *lua.State) int {
    var (
        payload []byte
    )
    p := L.ToGoStruct(1)
    if L.Type(2) == lua.LUA_TSTRING {
        payload = L.ToBytes(2)
    }
    if client, ok := p.(*kcpClient); ok {
        client.send(payload)
    } else {
        log.Println("转换 kcp-client 对象失败")
        L.PushString("convert error")
        return 1
    }
    return 0
}

func setHead(L *lua.State) int {
    p := L.ToGoStruct(1)
    b := L.ToBoolean(2)
    if client, ok := p.(*kcpClient); ok {
        client.withHead = b
    } else {
        L.PushString("convert error")
        return 1
    }
    return 0
}

func setTimeout(L *lua.State) int {
    p := L.ToGoStruct(1)
    sec := L.ToNumber(2)
    if client, ok := p.(*kcpClient); ok {
        client.timeout = time.Duration(sec * float64(time.Second))
    } else {
        L.PushString("convert error")
        return 1
    }
    return 0
}
