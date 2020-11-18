local cli = JSONRPCClient()
local err = cli.Connect('127.0.0.1:1334')

if err == nil then
    cli.Send(11, '你好', function(err, code, data)
        print('reply:', err, code, data)
    end)
else
    print('连接错误', err)
end