package lua_http

import (
    "encoding/base64"
    . "github.com/DGHeroin/golualib"
    "log"
    "net"
    "net/http"

    "github.com/DGHeroin/golua/lua"
)

var (
    initCode = `
local lib = lua_http
lua_http = nil

function HTTPServer()
    local self = {}
    local handler

    local function onRequest( r )
        if self.onRequest then 
            local rs = self.onRequest( r ) or {}
            return {
                statusCode = rs.statusCode or 200,
                body       = rs.body or '',
                headers    = rs.headers or {},
            }
        end
        
        return {
            statusCode = 200,
            body       = '',
            isBase64   = false,
            headers    = {
                ['Content-Type'] = 'text/plain'
            }
        }
    end

    function self.Init(addr)
        handler = lib.listen( addr, onRequest )
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

    // everything done
    L.SetGlobal("lua_http")

    err := L.DoString(initCode)
    if err != nil {
        log.Println(err)
    }
}

type httpHandler struct {
    ctx LuaContext
    L   *lua.State
    ref int
}

func (h *httpHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    L := h.L
    h.ctx.Run(func() {
        defer func() {
            if e:=recover(); e != nil {
                log.Println(e)
            }
        }()
        L.RawGeti(lua.LUA_REGISTRYINDEX, h.ref)

        // req
        {
            L.NewTable()
            {
               L.PushString("path")
               L.PushString(r.RequestURI)
               L.SetTable(-3)

                L.PushString("remoteAddr")
                L.PushString(r.RemoteAddr)
                L.SetTable(-3)

                L.PushString("host")
                L.PushString(r.Host)
                L.SetTable(-3)

                L.PushString("method")
                L.PushString(r.Method)
                L.SetTable(-3)

                L.PushString("method")
                L.PushInteger(r.ContentLength)
                L.SetTable(-3)

                L.PushString("header")
                {
                    L.NewTable()
                    for k, v := range r.Header {
                        L.PushString(k)
                        {
                            L.NewTable()
                            for idx, vv := range v {
                                L.PushInteger(int64(idx+1))
                                L.PushString(vv)
                                L.SetTable(-3)
                            }
                        }

                        L.SetTable(-3)
                    }
                }
                L.SetTable(-3)
            }
        }

        if err := L.Call(1, 1); err != nil {
            log.Println(err)
            return
        }
        if L.Type(-1) != lua.LUA_TTABLE {
            w.WriteHeader(http.StatusInternalServerError)
            return
        }
        L.GetField(1, "statusCode")
        if L.Type(-1) == lua.LUA_TNUMBER {
            statusCode := L.CheckInteger(-1)
            w.WriteHeader(statusCode)
        }
        isBase64 := false
        L.GetField(1, "isBase64")
        if L.Type(-1) == lua.LUA_TBOOLEAN {
            isBase64 = L.ToBoolean(-1)
        }

        L.GetField(1, "headers")
        if L.Type(-1) == lua.LUA_TTABLE {
            L.PushValue(-1)
            L.PushNil()
            for L.Next(-2) != 0 {
                L.PushValue(-2)
                if L.Type(-1) == lua.LUA_TSTRING && L.Type(-2) == lua.LUA_TSTRING {
                    key := L.ToString(-1)
                    value := L.ToString(-2)
                    w.Header().Add(key, value)
                }
                L.Pop(2)
            }
        }

        L.GetField(1, "body")
        if L.Type(-1) == lua.LUA_TSTRING {
            body := L.ToString(-1)
            if isBase64 {
                data, err := base64.StdEncoding.DecodeString(body)
                if err != nil {
                    log.Println(err)
                    return
                }
                w.Write(data)
            } else {
                w.Write([]byte(body))
            }
        }

    })
}

func listenServer(L *lua.State) int {
    addr := L.CheckString(1)
    ref := L.Ref(lua.LUA_REGISTRYINDEX)

    ctx := CheckLuaContext(L)
    handler := &httpHandler{
        ctx: ctx,
        L:   L,
        ref: ref,
    }

    go func() {
        defer func() {
            L.Unref(lua.LUA_REGISTRYINDEX, ref)
        }()
        ln, err := net.Listen("tcp", addr)
        if err != nil {
            log.Println(err)
            return
        }
        err = http.Serve(ln, handler)
        if err != nil {
            log.Println(err)
            return
        }
    }()
    L.PushGoStruct(handler)
    return 1
}
