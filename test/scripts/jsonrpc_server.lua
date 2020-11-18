local s = JSONRPCServer()
function s.onEvent( ... )
    print('==>', table.tostring({...}))
    return 33, 'qqq'
end
s.Init(':1334')
