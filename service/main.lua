local flying = require "flying"

local main = {}

function main:started()
    print("main started")

    flying.fork(function ()
        for i = 1, 10 do
            flying.sleep(100)
            print("main fork tick", i)
        end
    end)

    for i = 1, 5 do
        flying.sleep(200)
        print("main tick", i)
    end

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
