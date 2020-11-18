package main

import (
    "log"

    "github.com/DGHeroin/golualib/lua_jsonrpc"
    "github.com/DGHeroin/golualib/lua_kcp"
    "github.com/DGHeroin/golualib/lua_looper"
    "github.com/DGHeroin/golualib/lua_time"
    "github.com/DGHeroin/golualib/lua_websocket"

    "os"

    . "github.com/DGHeroin/golualib"
    "github.com/DGHeroin/golualib/lua_http"
)

func main() {
    log.SetFlags(log.LstdFlags | log.Lshortfile)
    ctx := NewDefaultContext()
    L := ctx.LuaState()
    lua_http.Register(L)
    lua_looper.Register(L)
    lua_time.Register(L)
    lua_websocket.Register(L)
    lua_kcp.Register(L)
    lua_jsonrpc.Register(L)
    if len(os.Args) == 2 {
        if fi, err := os.Stat(os.Args[1]); err == nil && !fi.IsDir() {
            if err := L.DoFile(os.Args[1]); err != nil {
                log.Println(err)
            }
            ctx.WaitQuit()
            return
        }
    }
    log.Println("no input file")
}
