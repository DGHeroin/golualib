local server = KCPServer()

local x = {}
function server.onEvent(eventType, id, client, msgType, msgData)
    print(eventType, id, client, msgType, msgData)
    if eventType == 1 then
        server.SetTimeout(client, 5)
        server.SetHead(client, true)
        return
    end
    server.Send(client, 1, 'world')
    -- if not x[id] then
    --     x[id] = client
    --     Looper.AfterFunc(2, function( ... )
    --         server.Close(client)
    --     end)
    -- end
end

local addr = ':1234'
print('serve on', addr)
server.Init(addr)


