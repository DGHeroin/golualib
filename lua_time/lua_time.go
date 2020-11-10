package lua_time

import (
    "time"

    "github.com/DGHeroin/golua/lua"
)

var (
    startTime time.Time
)

func init() {
    startTime = time.Now()
}

func Register(L *lua.State) {
    L.CreateTable(0, 1)

    L.PushGoFunction(timeNow)
    L.SetGlobal("TimeNow")

    L.PushGoFunction(timeSinceStart)
    L.SetGlobal("TimeSinceStart")
}
func timeNow(L *lua.State) int {
    now := time.Now()
    L.PushInteger(now.UnixNano())
    return 1
}

func timeSinceStart(L *lua.State) int {
    dur := time.Now().Sub(startTime)
    L.PushInteger(dur.Nanoseconds())
    return 1
}
