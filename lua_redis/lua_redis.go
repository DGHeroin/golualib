package lua_redis

import (
    "context"
    "crypto/tls"
    "github.com/DGHeroin/golua/lua"
    . "github.com/DGHeroin/golualib"
    "github.com/go-redis/redis/v8"
    "log"
)

var (
    initCode = `
local lib = lua_redis
lua_http = nil

function RedisClient()
    local self = {}
    local handler

    function self.Connect(addr, username, password, db)
        username = username or ''
        password = password or ''
        db       = db       or 0
        handler, err = lib.connect( addr, username, password, db )
        return err
    end

    function self.Get(key, cb)
        lib.get(handler, key, function(err, val) 
            if cb then 
                cb(err, val) 
            end 
        end)
    end

    function self.Set(key, val, cb)
        lib.set(handler, key, val, function(err, val) 
            if cb then 
                cb(err, val) 
            end 
        end)
    end

    return self
end

`
)

func Register(L *lua.State) {
    L.CreateTable(0, 1)

    //  connect
    L.PushString("connect")
    L.PushGoFunction(connect)
    L.SetTable(-3)

    //  get
    L.PushString("get")
    L.PushGoFunction(get)
    L.SetTable(-3)

    //  get
    L.PushString("set")
    L.PushGoFunction(set)
    L.SetTable(-3)

    // everything done
    L.SetGlobal("lua_redis")

    err := L.DoString(initCode)
    if err != nil {
        log.Println(err)
    }
}

func connect(L *lua.State) int {
    var (
        addr     string
        username string
        password string
        db       int
        certFile string
        keyFile  string
    )
    opt := &redis.Options{
        Addr:     addr,
        Username: username,
        Password: password,
        DB:       db,
    }
    addr = L.CheckString(1)
    username = L.CheckString(2)
    password = L.CheckString(3)
    db = L.CheckInteger(4)
    if L.GetTop() == 6 { //  包含 tls
        certFile = L.CheckString(5)
        keyFile = L.CheckString(6)
        cert, err := tls.LoadX509KeyPair(certFile, keyFile)
        if err != nil {
            L.PushNil()
            L.PushString(err.Error())
            return 2
        }
        opt.TLSConfig = &tls.Config{
            InsecureSkipVerify: true,
            Certificates:       []tls.Certificate{cert},
        }
    }

    cli := redis.NewClient(opt)
    if cmd := cli.Ping(context.Background()); cmd.Err() != nil {
        _ = cli.Close()
        L.PushNil()
        L.PushString(cmd.Err().Error())
        return 2
    }
    L.PushGoStruct(cli)
    L.PushNil()
    return 2
}

func get(L *lua.State) int {
    L.CheckType(2, lua.LUA_TSTRING)
    L.CheckType(3, lua.LUA_TFUNCTION)

    ptr := L.ToGoStruct(1)
    key := L.ToString(2)
    ref := L.Ref(lua.LUA_REGISTRYINDEX)
    ctx := CheckLuaContext(L)
    cli, ok := ptr.(*redis.Client)

    if !ok {
        L.RawGeti(lua.LUA_REGISTRYINDEX, ref)

        L.PushString("redis client pointer convert failed.")
        if err := L.Call(1, 0); err != nil {
            log.Println(err)
        }
        L.Unref(lua.LUA_REGISTRYINDEX, ref)
        return 1
    }

    cmd := cli.Get(context.Background(), key)
    if cmd.Err() != nil { // 发生错误
        ctx.Run(func() {
            L.RawGeti(lua.LUA_REGISTRYINDEX, ref)

            L.PushString(cmd.Err().Error())
            if err := L.Call(1, 0); err != nil {
                log.Println(err)
            }
            L.Unref(lua.LUA_REGISTRYINDEX, ref)
        })
    } else {
        ctx.Run(func() {
            data, err := cmd.Bytes()
            L.RawGeti(lua.LUA_REGISTRYINDEX, ref)

            if err != nil {
                L.PushString(err.Error())
                L.PushNil()
            } else {
                L.PushNil()
                if data == nil || len(data) == 0 {
                    L.PushNil()
                } else {
                    L.PushBytes(data)
                }
            }

            if err := L.Call(2, 0); err != nil {
                log.Println(err)
            }
            L.Unref(lua.LUA_REGISTRYINDEX, ref)
        })
    }

    return 0
}

func set(L *lua.State) int {
    L.CheckType(2, lua.LUA_TSTRING)
    L.CheckType(3, lua.LUA_TSTRING)
    L.CheckType(4, lua.LUA_TFUNCTION)

    ptr := L.ToGoStruct(1)
    key := L.ToString(2)
    val := L.ToBytes(3)
    ref := L.Ref(lua.LUA_REGISTRYINDEX)
    ctx := CheckLuaContext(L)
    cli, ok := ptr.(*redis.Client)
    if !ok {
        L.RawGeti(lua.LUA_REGISTRYINDEX, ref)

        L.PushString("redis client pointer convert failed.")
        if err := L.Call(1, 0); err != nil {
            log.Println(err)
        }
        L.Unref(lua.LUA_REGISTRYINDEX, ref)
        return 1
    }

    defer func() {
        if e := recover(); e != nil {
            log.Println(e)
        }
    }()
    cmd := cli.Set(context.Background(), key, val, 0)

    if cmd.Err() != nil {
        ctx.Run(func() {
            L.RawGeti(lua.LUA_REGISTRYINDEX, ref)
            L.PushString(cmd.Err().Error())
            if err := L.Call(1, 0); err != nil {
                log.Println(err)
            }
            L.Unref(lua.LUA_REGISTRYINDEX, ref)
        })
        return 0
    }

    ctx.Run(func() {
        L.RawGeti(lua.LUA_REGISTRYINDEX, ref)
        L.PushNil()
        L.PushString(cmd.String())
        if err := L.Call(2, 0); err != nil {
            log.Println(err)
        }
        L.Unref(lua.LUA_REGISTRYINDEX, ref)
    })

    return 0
}
