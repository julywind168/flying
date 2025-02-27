local flying = require "flying"

local ping = {}

function ping:init()
    print("ping service init")
end

function ping:started()
    print("ping service started")
    print("hello", flying.getenv("hello"))
    flying.fork(function ()
        for i = 1, 5 do
            flying.sleep(200)
            print("tick", i)
        end
    end)
    flying.timeout(2000, function ()
        print("timeout")
    end)
end

function ping:message(cmd)
    if cmd == "ping" then
        return "PONG"
    else
        print("PING " .. tostring(cmd))
    end
end

function ping:stopping()
    print("ping service stopping")
end

function ping:stopped()
    print("ping service stopped")
end

return flying.service(ping)
