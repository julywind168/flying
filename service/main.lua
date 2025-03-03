local flying = require "flying"

local main = {}

function main:started()
    print("main started", flying.starttime(), flying.now(), flying.time())

    flying.fork(function ()
        for i = 1, 5 do
            flying.sleep(100)
            print("main tick", i)
        end
    end)

    -- flying.spawn("test_mongo", "service/test_mongo.lua");
    flying.spawn("echo", "service/echo.lua")
    flying.spawn("ping", "service/ping.lua")
    print(flying.call("ping", "PING"))
end

function main:message(source, message)
    print("main message", source, message)
end

function main:stopped()
    print("main stopped")
end

flying.start(main)
