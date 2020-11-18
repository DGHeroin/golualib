package lua_jsonrpc

import (
    "log"
    "net"
    "net/rpc"
    "net/rpc/jsonrpc"
    "sync"

    "github.com/DGHeroin/golua/lua"
    . "github.com/DGHeroin/golualib"
)

var (
    initCode = `
local lib = lua_jsonrpc
lua_http = nil
function JSONRPCClient()
    local self = {}
    local handler
    function self.Connect(addr)
        handler, err = lib.connect(addr)
        return err
    end
    function self.Send(code, data, cb)
        lib.send(handler, code, data, function(...) cb(...) end)
    end
    return self
end
function JSONRPCServer()
    local self = {}
    local handler

    local function onEvent( ... )
        local code
        local data 
        if self.onEvent then
            code, data = self.onEvent( ... )
        end
        code = code or -1
        data = data or ''
        
        return code, data
    end

    function self.Init(addr)
        handler, err = lib.listen( addr, onEvent )
        return err
    end

    function self.Close()
        if not handler then return end
        lib.close( handler )
        handler = nil
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
    L.PushGoFunction(closeServer)
    L.SetTable(-3)

    // client
    L.PushString("connect")
    L.PushGoFunction(clientConnect)
    L.SetTable(-3)

    L.PushString("send")
    L.PushGoFunction(clientSend)
    L.SetTable(-3)


    // everything done
    L.SetGlobal("lua_jsonrpc")

    err := L.DoString(initCode)
    if err != nil {
        log.Println(err)
    }
}

type Handler struct {
    ln  net.Listener
    ref int
    ctx LuaContext
}

type Args struct {
    Code int
    Data []byte
}

func listenServer(L *lua.State) int {
    addr := L.CheckString(1)
    ref := L.Ref(lua.LUA_REGISTRYINDEX)
    ctx := CheckLuaContext(L)

    ln, err := net.Listen("tcp", addr)
    if err != nil {
        L.PushNil()
        L.PushString(err.Error())
        return 2
    }
    s := &Handler{
        ln:  ln,
        ref: ref,
        ctx: ctx,
    }
    rpc.Register(s)
    go s.serve()
    L.PushGoStruct(s)
    L.PushNil()
    return 2
}

func closeServer(L *lua.State) int {
    p := L.ToGoStruct(1)
    if s, ok := p.(*Handler); ok {
        s.ln.Close()
    }
    return 0
}

func (s *Handler) serve() {
    serveConn := func(conn net.Conn) {
        defer conn.Close()
        rpc.ServeCodec(jsonrpc.NewServerCodec(conn))
    }

    for {
        conn, err := s.ln.Accept()
        if err != nil {
            log.Println(err)
            return
        }
        go serveConn(conn)
    }

    s.ctx.Run(func() {
        L := s.ctx.LuaState()
        L.Unref(lua.LUA_REGISTRYINDEX, s.ref)
    })
}

func (s *Handler) Invoke(args *Args, reply *Args) error {
    var (
        err error
        wg  sync.WaitGroup
    )
    wg.Add(1)
    s.ctx.Run(func() {
        L := s.ctx.LuaState()
        L.RawGeti(lua.LUA_REGISTRYINDEX, s.ref)
        L.PushInteger(int64(args.Code))
        L.PushBytes(args.Data)
        if err = L.Call(2, 2); err != nil {
            log.Println(err)
        } else {
            var (
                code = -1
                data []byte
            )
            if L.Type(2) == lua.LUA_TNUMBER {
                code = L.CheckInteger(2)
            }
            if L.Type(3) == lua.LUA_TSTRING {
                data = L.ToBytes(3)
            }
            (*reply).Code = code
            (*reply).Data = data
        }
        wg.Done()
    })
    wg.Wait()
    return err
}

// client
type client struct {
    conn *rpc.Client
}

func clientConnect(L *lua.State) int {
    addr := L.CheckString(1)
    conn, err := net.Dial("tcp", addr)
    if err != nil {
        L.PushNil()
        L.PushString(err.Error())
        return 2
    }
    c := rpc.NewClientWithCodec(jsonrpc.NewClientCodec(conn))
    cli := &client{
        conn: c,
    }
    L.PushGoStruct(cli)
    L.PushNil()
    return 2
}

func clientSend(L *lua.State) int {
    p := L.ToGoStruct(1)
    code := L.CheckInteger(2)
    data := L.ToBytes(3)
    ref := L.Ref(lua.LUA_REGISTRYINDEX)

    ctx := CheckLuaContext(L)
    if cli, ok := p.(*client); ok {
        go func() {
            args := &Args{Code: code, Data: data}
            var reply Args
            client := cli.conn
            err := client.Call("Handler.Invoke", args, &reply)

            ctx.Run(func() {
                L.RawGeti(lua.LUA_REGISTRYINDEX, ref)
                if L.Type(-1) != lua.LUA_TFUNCTION {
                    return
                }
                if err == nil {
                    L.PushNil()
                } else {
                    L.PushString(err.Error())
                }

                L.PushInteger(int64(reply.Code))
                L.PushBytes(reply.Data)
                if err := L.Call(3, 0); err != nil {
                    log.Println(err)
                }
                L.Unref(lua.LUA_REGISTRYINDEX, ref)
            })
        }()
    }
    return 0
}
