package lua_looper

import (
    . "github.com/DGHeroin/golualib"
    "log"
    "time"

    "github.com/DGHeroin/golua/lua"

)

func Register(L *lua.State) {
    L.CreateTable(0, 1)

    //  server
    L.PushString("New")
    L.PushGoFunction(newLooper)
    L.SetTable(-3)

    L.PushString("Stop")
    L.PushGoFunction(stopLooper)
    L.SetTable(-3)

    L.PushString("AfterFunc")
    L.PushGoFunction(afterFunc)
    L.SetTable(-3)

    // everything done
    L.SetGlobal("loop")

    err := L.DoString(`
local l = loop
local timeCounter = 0
function LoopTimeCount_ns()
    return l.LoopTimeCount_ns()
end

local function LoopTimeCounter( ns )
    timeCounter = timeCounter + ns
end

function LuaLoop()
    local self = {}
    local loop
    local updateList = {}
    local function onUpdate()
        for _, cb in pairs(updateList) do
            cb()
        end
    end
    function self.Start(ms)
        loop = l.New(ms, onUpdate, LoopTimeCounter)
    end
    function self.Stop()
        if loop then
            l.Stop(loop)
        end
        loop = nil
    end
    function self.AfterFunc(sec, cb)
        l.AfterFunc(sec, cb)
    end
    function self.AddUpdate(cb)
        updateList[cb] = cb
    end
    function self.RemoveUpdate(cb)
        updateList[cb] = nil
    end
    return self
end

-- default looper
Looper = LuaLoop()
loop = nil
`)
    if err != nil {

    }
}
type loop struct {
    callbackRef    int
    counterRef int
    rate           int
    ticker         *time.Ticker
    L              *lua.State
    ctx            LuaContext
}

func newLooper(L *lua.State) int {
    l := &loop{}
    l.rate = L.CheckInteger(1)

    l.counterRef = L.Ref(lua.LUA_REGISTRYINDEX)
    L.SetTop(2)
    l.callbackRef = L.Ref(lua.LUA_REGISTRYINDEX)

    l.L = L
    l.ctx = CheckLuaContext(L)

    l.Start()

    L.PushGoStruct(l)
    return 1
}

func stopLooper(L *lua.State) int {
    looper := checkLooper(L, 1)
    looper.Stop()
    return 0
}

func checkLooper(L *lua.State, i int) *loop {
    ptr := L.ToGoStruct(i)
    if l, ok := ptr.(*loop); ok {
        return l
    }
    return nil
}

func (l *loop) Start() {
    go func() {
        l.ticker = time.NewTicker(time.Millisecond * time.Duration(l.rate))
        L := l.L
        for {
            <-l.ticker.C
            l.ctx.Run(func() {
                startTime := time.Now()
                // tick
                L.RawGeti(lua.LUA_REGISTRYINDEX, l.callbackRef)
                if err := L.Call(0, 0); err != nil {
                    log.Println(err)
                }
                // calc
                elapsedNs := time.Now().Sub(startTime).Nanoseconds()
                L.RawGeti(lua.LUA_REGISTRYINDEX, l.counterRef)
                L.PushInteger(elapsedNs)
                if err := L.Call(1, 0); err != nil {
                    log.Println(err)
                }
            })
        }
    }()
}
func (l *loop) Stop() {
    l.ticker.Stop()
}

func afterFunc(L *lua.State) int {
    sec := L.CheckNumber(1)
    ref := L.Ref(lua.LUA_REGISTRYINDEX)
    ctx := CheckLuaContext(L)
    L.RawGeti(lua.LUA_REGISTRYINDEX, ref)
    if L.Type(-1) != lua.LUA_TFUNCTION {
        log.Println("AfterFunc not function")
        return 0
    }

    dur := time.Duration(sec * float64(time.Second))
    time.AfterFunc(dur, func() {
        ctx.Run(func() {
            L.RawGeti(lua.LUA_REGISTRYINDEX, ref)
            if L.Type(-1) == lua.LUA_TFUNCTION {
                if err := L.Call(0, 0); err != nil {
                    log.Println(err)
                }
                L.Unref(lua.LUA_REGISTRYINDEX, ref)
            }
        })
    })
    return 0
}