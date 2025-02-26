local flying = require "flying"

local function main()
    print("server start at", flying.starttime())
    flying.setenv("hello", "world")
    flying.newservice("ping", "service/ping.lua")
    flying.send("ping", "How are you?")
    local pong = flying.call("ping", "ping")
    print(pong)
    for i = 1, 10 do
        flying.sleep(200)
        print("tick", i)
    end
    print("bye")
end

return flying.oneshotservice(main)
