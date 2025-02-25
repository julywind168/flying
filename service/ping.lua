local flying = require "flying"

local ping = {}

function ping:init()
    print("ping service init")
end

function ping:started()
    print("ping service started")
    print("hello", flying.getenv("hello"))
end

function ping:message(cmd)
    if cmd == "ping" then
        return "pong"
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
