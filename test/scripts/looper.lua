local l = LuaLoop()
local count = 0
l.Start(1000, function()
    print('tick', count)
    count = count + 1
    if count > 3 then
        l.Stop()
    end
end)
