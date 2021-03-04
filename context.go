package golualib

import (
    "log"
    "os"
    "os/signal"
    "runtime"
    "runtime/debug"
    "sync"
    "sync/atomic"
    "time"

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

var (
    mutex sync.Mutex
)

func CheckLuaContext(L *lua.State) LuaContext {
    mutex.Lock()
    defer mutex.Unlock()
    L.GetGlobal(ContextGlobalName)
    ptr := L.ToGoStruct(-1)
    if c, ok := ptr.(LuaContext); ok {
        return c
    }
    debug.PrintStack()
    return nil
}

type luaContext struct {
    mutex  sync.Mutex
    L      *lua.State
    cbChan chan func()
}

func NewDefaultContext(L *lua.State) LuaContext {
    ctx := &luaContext{
        cbChan: make(chan func()),
    }
    if L == nil {
        L = lua.NewState()
    }
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
    ctx.cbChan <- cb
}

func (ctx *luaContext) LuaState() *lua.State {
    return ctx.L
}
func (ctx *luaContext) WaitQuit() {
    c := make(chan os.Signal, 1)
    signal.Notify(c, os.Interrupt, os.Kill)
    co := int64(0)
    go func() {
        for {
           select {
           case cb := <- ctx.cbChan:
               atomic.AddInt64(&co, 1)
               invoke(cb)
               atomic.AddInt64(&co, -1)
           }
        }
    }()
    go func() {
        for {
            log.Println("co num: ", atomic.LoadInt64(&co))
            time.Sleep(time.Second*10)
            runtime.GC()
            debug.FreeOSMemory()
        }
    }()
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

func invoke(cb func())  {
    defer func() {
        if e := recover(); e != nil {
            log.Println(e)
            os.Exit(1)
        }
    }()
    cb()
}