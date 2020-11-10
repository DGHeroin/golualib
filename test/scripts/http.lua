local server = HTTPServer()

function server.onRequest(r)
    print('onRequest:', table.tostring(r))
    return {
        statusCode = 201,
        body       = 'Hello World!!!',
        isBase64   = false,
        headers    = {
            ['Content-Type'] = 'text/html',
            cc=function ( ... )
                -- body
            end
        }
    }
end
server.Init(':8080')