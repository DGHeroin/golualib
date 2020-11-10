package golualib

import (
    "log"
    "os"
    "os/signal"
    "runtime/debug"
    "sync"

    "github.com/DGHeroin/golua/lua"
)

type LuaContext interface {
    Run(func())
    LuaState() *lua.State
    WaitQuit()
}

const (
    ContextGlobalName = "_lua_context_"
)

func CheckLuaContext(L *lua.State) LuaContext {
    L.GetGlobal(ContextGlobalName)
    ptr := L.ToGoStruct(-1)
    if c, ok := ptr.(LuaContext); ok {
        return c
    }
    debug.PrintStack()
    return nil
}

type luaContext struct {
    mutex sync.Mutex
    L     *lua.State
}

func NewDefaultContext() LuaContext {
    ctx := &luaContext{}
    L := lua.NewState()
    ctx.L = L
    L.OpenLibs()
    L.OpenGoLibs()

    L.PushGoStruct(ctx)
    L.SetGlobal(ContextGlobalName)
    if err := L.DoString(LuaUtilsCode); err != nil {
        log.Println(err)
    }
    return ctx
}

func (ctx *luaContext) Run(cb func()) {
    ctx.mutex.Lock()
    cb()
    ctx.mutex.Unlock()
}

func (ctx *luaContext) LuaState() *lua.State {
    return ctx.L
}
func (ctx *luaContext) WaitQuit() {
    c := make(chan os.Signal, 1)
    signal.Notify(c, os.Interrupt, os.Kill)
    sig := <-c

    wg := sync.WaitGroup{}
    wg.Add(1)
    ctx.Run(func() {
        L := ctx.L
        L.GetGlobal("OnApplicationQuit")
        if L.IsFunction(-1) {
            L.PushString(sig.String())
            L.Call(0, 0)
        }
        wg.Done()
        os.Exit(0)
    })
    wg.Wait()
}
