package lua_kcp

import (
    "github.com/DGHeroin/golua/lua"
    "sync"
)

func (c *kcpHandler) OnConnect(conn *Conn) bool {
    ctx := c.ctx
    L := c.ctx.LuaState()
    ref := c.ref
    id := c.id
    rs := true
    var wgAccept sync.WaitGroup
    wgAccept.Add(1)
    ctx.Run(func() {
        L.RawGeti(lua.LUA_REGISTRYINDEX, ref)
        L.PushInteger(EventTypeConnected)
        L.PushInteger(int64(id))
        L.PushGoStruct(conn)
        if err := L.Call(3, 0); err != nil {
            c.OnError(conn, err)
            rs = false
        }
        wgAccept.Done()
    })
    return rs
}

func (c *kcpHandler) OnMessage(conn *Conn, data []byte) bool {
    ctx := c.ctx
    L := c.ctx.LuaState()
    ref := c.ref
    id := c.id
    ctx.Run(func() {
        L.RawGeti(lua.LUA_REGISTRYINDEX, ref)
        L.PushInteger(EventTypeData)
        L.PushInteger(int64(id))
        L.PushGoStruct(conn)
        if data == nil || len(data) == 0 {
            L.PushNil()
        } else {
            L.PushBytes(data)
        }

        if err := L.Call(4, 0); err != nil {
            c.OnError(conn, err)
        }
    })
    return true
}

func (c *kcpHandler) OnClose(conn *Conn) {
    ctx := c.ctx
    L := c.ctx.LuaState()
    ref := c.ref
    id := c.id
    ctx.Run(func() {
        L.RawGeti(lua.LUA_REGISTRYINDEX, ref)
        L.PushInteger(EventTypeClose)
        L.PushInteger(int64(id))
        L.PushGoStruct(conn)
        if err := L.Call(3, 0); err != nil {
            c.OnError(conn, err)
        }
    })
}

func (c *kcpHandler) OnError(conn *Conn, err error) {
    //log.Println(conn, err)
}