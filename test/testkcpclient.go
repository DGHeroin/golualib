package main

import (
    "encoding/binary"
    "log"
    "time"

    "github.com/xtaci/kcp-go"
)

func main()  {
    conn, err := kcp.Dial("127.0.0.1:1234")
    if err != nil {
        log.Println(err)
        return
    }

    defer conn.Close()
    for {
        _, err := conn.Write(combine([]byte(time.Now().String())))
        if err != nil {
            log.Println(err)
            return
        }
        time.Sleep(time.Second)
    }
}
func combine(data[]byte) []byte {
    header := make([]byte, 4)
    binary.BigEndian.PutUint32(header, uint32(len(data)))
    return append(header, data...)
}
