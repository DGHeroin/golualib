package main

import (
    "log"
    "net"
    "net/rpc"
    "net/rpc/jsonrpc"

    "github.com/DGHeroin/golualib/lua_jsonrpc"
)

func main() {
    conn, err := net.Dial("tcp", ":1334")
    if err != nil {
        log.Fatal("dial error:", err)
    }

    client := rpc.NewClientWithCodec(jsonrpc.NewClientCodec(conn))

    args := &lua_jsonrpc.Args{7, []byte("hello world!")}
    var reply lua_jsonrpc.Args
    err = client.Call("Handler.Invoke", args, &reply)
    if err != nil {
        log.Fatal("error:", err)
    }
    log.Printf("%+v =>%+v", args, reply)
}
