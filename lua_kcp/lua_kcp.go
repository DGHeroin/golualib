package lua_kcp

import (
    "log"
    "net"
    "sync"
    "sync/atomic"
    "time"

    "github.com/DGHeroin/golua/lua"
    . "github.com/DGHeroin/golualib"

    "github.com/xtaci/kcp-go"
)

const (
    EventTypeConnected = 1
    EventTypeData      = 2
    EventTypeClose     = 3
)

var (
    initCode = `
local lib = lua_kcp
lua_http = nil

function KCPServer()
    local self = {}
    local handler
    local timeout
    local function onEvent( evtType, id, client, msgData )
        if self.onEvent then
            self.onEvent(evtType, id, client, msgData)
        end
    end

    function self.Init(addr)
        handler = lib.listen( addr, onEvent )
        if timeout then lib.setTimeout(handler, timeout) end
    end

    function self.Close(client)
        if not handler then return end
        lib.close( client )
    end
    
    function self.Send(client, t, d)
        if not handler then return end
        return lib.send(client, t, d)
    end

    function self.SetHead(client, b)
        lib.setHead(client, b)
    end

    function self.SetTimeout(sec)
        if handler then
            lib.setTimeout(handler, sec)
        else
            timeout = sec
        end
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
    ctx       LuaContext
    L         *lua.State
    ref       int
    id        uint32
    waitGroup *sync.WaitGroup
    exitChan  chan struct{}
    timeout   time.Duration
}

func listenServer(L *lua.State) int {
    addr := L.CheckString(1)
    ref := L.Ref(lua.LUA_REGISTRYINDEX)

    ctx := CheckLuaContext(L)
    handler := &kcpHandler{
        ctx:       ctx,
        L:         L,
        ref:       ref,
        waitGroup: &sync.WaitGroup{},
        exitChan:  make(chan struct{}),
        timeout:   time.Second * 10,
    }

    go func() {
        defer func() {
            if e := recover(); e != nil {
                log.Println(e)
            }
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

func handlerFunc(h *kcpHandler, conn net.Conn) {
    c := &Conn{
        conn:              conn,
        id:                atomic.AddUint32(&h.id, 1),
        closeChan:         make(chan struct{}),
        srv:               h,
        packetReceiveChan: make(chan []byte, 10),
        packetSendChan:    make(chan []byte, 10),
        withHead:          true,
        timeout:           h.timeout,
    }
    log.Println("timeout", c.timeout)
    c.SetCallback(h)

    go func() { // read message
        for !c.IsClosed() {
            select {
            case data := <-c.packetReceiveChan:
                if atomic.CompareAndSwapInt32(&c.openFlag, 0, 1) {
                    h.OnConnect(c)
                    h.OnMessage(c, data)
                    return
                }
                h.OnMessage(c, data)
            }
        }
    }()
    asyncDo(c.readLoop, h.waitGroup)
    asyncDo(c.writeLoop, h.waitGroup)

}

func closeConn(L *lua.State) int {
    p := L.ToGoStruct(1)
    if client, ok := p.(*Conn); ok {
        go client.Close()
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
    if client, ok := p.(*Conn); ok {
        st, err := client.Send(payload)
        L.PushInteger(int64(st))
        if err != nil {
            L.PushString(err.Error())
        } else {
            L.PushNil()
        }
        return 2
    } else {
        L.PushInteger(-1)
        L.PushString("convert error")
        return 2
    }
}

func setHead(L *lua.State) int {
    p := L.ToGoStruct(1)
    b := L.ToBoolean(2)
    if client, ok := p.(*Conn); ok {
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
    if h, ok := p.(*kcpHandler); ok {
        h.timeout = time.Duration(sec * float64(time.Second))
    } else {
        L.PushString("convert error")
        return 1
    }
    return 0
}
