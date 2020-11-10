local server = WSServer()

-- eventType 1 new
--           2 data
--           3 close
local x = {}
function server.onEvent(eventType, id, client, msgType, msgData)
    print(eventType, id, client, msgType, msgData)
    server.Send(client, 1, 'world')
    if not x[id] then
        x[id] = client
        Looper.AfterFunc(2, function( ... )
            server.Close(client)
        end)
    end
end

server.Init(':80')
