package lua_websocket

import (
    "log"
    "net/http"
    "sync"
    "sync/atomic"

    "github.com/DGHeroin/golua/lua"
    . "github.com/DGHeroin/golualib"
    "github.com/gin-gonic/gin"
    "github.com/gorilla/websocket"
)

var (
    initCode = `
local lib = lua_ws
lua_http = nil

function WSServer()
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

    // everything done
    L.SetGlobal("lua_ws")

    err := L.DoString(initCode)
    if err != nil {
        log.Println(err)
    }
}

type wsHandler struct {
    ctx LuaContext
    L   *lua.State
    ref int
    id  uint32
}

func listenServer(L *lua.State) int {
    addr := L.CheckString(1)
    ref := L.Ref(lua.LUA_REGISTRYINDEX)

    ctx := CheckLuaContext(L)
    handler := &wsHandler{
        ctx: ctx,
        L:   L,
        ref: ref,
    }

    go func() {
        defer func() {
            L.Unref(lua.LUA_REGISTRYINDEX, ref)
        }()
        gin.SetMode(gin.ReleaseMode)
        r := gin.New()
        r.GET("/ws", func(c *gin.Context) {
            handlerFunc(handler, c)
        })
        r.GET("/pong", func(c *gin.Context) {
            c.JSON(http.StatusOK, gin.H{"message": "pong"})
        })
        r.Run(addr)
    }()

    L.PushGoStruct(handler)
    return 1
}

type wsClient struct {
    close func()
    send  func(msgType int, payload []byte) error
    id    uint32
}

func handlerFunc(h *wsHandler, c *gin.Context) {
    var (
        L      = h.L
        ref    = h.ref
        wg     sync.WaitGroup
        client = &wsClient{}
    )

    ctx := CheckLuaContext(L)
    conn, err := (&websocket.Upgrader{CheckOrigin: func(r *http.Request) bool {
        return true
    }}).Upgrade(c.Writer, c.Request, nil)
    if err != nil {
        http.NotFound(c.Writer, c.Request)
        return
    }
    client.id = atomic.AddUint32(&h.id, 1)
    client.close = func() {
        conn.Close()
    }
    client.send = func(msgType int, payload []byte) error {
        return conn.WriteMessage(msgType, payload)
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
    // read
    wg.Add(1)
    go func() {
        defer func() {
            wg.Done()
        }()
        for {
            msgType, data, err := conn.ReadMessage()
            if err != nil {
                return
            }
            var wgLua sync.WaitGroup
            wgLua.Add(1)
            ctx.Run(func() {
                L.RawGeti(lua.LUA_REGISTRYINDEX, ref)
                L.PushInteger(2)
                L.PushInteger(int64(client.id))
                L.PushGoStruct(client)
                L.PushInteger(int64(msgType))
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
    //wg.Add(1)
    //go func() {
    //    defer func() {
    //        fmt.Println("退出写")
    //        wg.Done()
    //    }()
    //    for {
    //        select {
    //
    //        }
    //    }
    //}()
    wg.Wait()
}

func closeConn(L *lua.State) int {
    p := L.ToGoStruct(1)
    if client, ok := p.(*wsClient); ok {
        client.close()
    } else {
        log.Println("转换 client 对象失败")
    }
    return 0
}

func sendConn(L *lua.State) int {
    var (
        msgType int
        payload []byte
    )
    p := L.ToGoStruct(1)
    msgType = L.CheckInteger(2)
    if L.Type(3) == lua.LUA_TSTRING {
        payload = L.ToBytes(3)
    }
    if client, ok := p.(*wsClient); ok {
        client.send(msgType, payload)
    } else {
        log.Println("转换 client 对象失败")
    }
    return 0
}
